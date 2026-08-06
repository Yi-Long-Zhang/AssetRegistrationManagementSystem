package service

import (
	"errors"
	"sort"
	"strings"
	"time"

	"asset-registration-management-system/backend/internal/model"

	"gorm.io/gorm"
)

// reconcile 将扫描结果与资产台账比对并落库：
// - 未匹配资产的存活主机 → new（新增）
// - 已匹配且端口/主机名/OS 变化 → changed（变更）
// - 已匹配、无字段变化且此前非在线 → online（恢复在线）
// - 已匹配、无任何变化 → none
// - 资产此前在线、本轮未出现且上一轮同样未出现 → offline（连续 2 轮离线）
// 同时更新运行记录统计。
func (s *DiscoveryService) reconcile(run *model.DiscoveryRun, results []ScanResult, now time.Time) error {
	var assets []model.Asset
	if err := s.DB.Find(&assets).Error; err != nil {
		return err
	}
	byIP := make(map[string]*model.Asset, len(assets))
	for i := range assets {
		byIP[assets[i].IP] = &assets[i]
		if assets[i].ManagementIP != "" {
			byIP[assets[i].ManagementIP] = &assets[i]
		}
	}

	seen := make(map[string]bool, len(results))
	hosts := make([]model.DiscoveredHost, 0, len(results))
	for _, r := range results {
		if r.Status != "up" {
			continue
		}
		seen[r.IP] = true
		change := model.DiscoveryChangeNew
		var matchedID *uint
		var diff string
		if asset, ok := byIP[r.IP]; ok {
			matchedID = &asset.ID
			change, diff = classifyChange(asset, r)
		}
		hosts = append(hosts, model.DiscoveredHost{
			RunID:          run.ID,
			IP:             r.IP,
			Hostname:       r.Hostname,
			Status:         r.Status,
			OpenPorts:      strings.Join(r.OpenPorts, ","),
			Services:       joinLines(r.Services),
			OS:             r.OS,
			ChangeType:     change,
			MatchedAssetID: matchedID,
			DiffSummary:    diff,
		})
	}

	// 离线判定：仅对“本轮未出现”的资产，且上一轮同样未出现时才标记（连续 2 轮）
	var offline []model.DiscoveredHost
	for ip, asset := range byIP {
		if seen[ip] {
			continue
		}
		if asset.OnlineStatus != model.AssetOnlineStatusOnline && asset.OnlineStatus != model.AssetOnlineStatusUnknown {
			continue // 未纳管/已退休等不参与离线判定
		}
		if asset.LastSeenAt == nil {
			continue // 从未被扫描发现过，无从判定离线
		}
		absentPrev, err := absentLastRound(s.DB, run.RuleID, ip, run.ID)
		if err != nil {
			return err
		}
		if !absentPrev {
			continue // 上一轮还在，本轮仅缺席一次，等待下一轮确认
		}
		offline = append(offline, model.DiscoveredHost{
			RunID:          run.ID,
			IP:             ip,
			Status:         "down",
			ChangeType:     model.DiscoveryChangeOffline,
			MatchedAssetID: &asset.ID,
			DiffSummary:    "连续两轮未发现主机",
		})
	}

	if len(hosts) > 0 {
		if err := s.DB.Create(&hosts).Error; err != nil {
			return err
		}
	}
	if len(offline) > 0 {
		if err := s.DB.Create(&offline).Error; err != nil {
			return err
		}
	}

	run.TotalHosts = len(hosts)
	run.NewCount = countChange(hosts, model.DiscoveryChangeNew)
	run.ChangedCount = countChange(hosts, model.DiscoveryChangeChanged)
	run.OnlineCount = countChange(hosts, model.DiscoveryChangeOnline)
	run.OfflineCount = len(offline)
	return nil
}

// classifyChange 对比资产当前字段与扫描结果，返回变更类型与 diff 摘要。
func classifyChange(asset *model.Asset, r ScanResult) (model.DiscoveryChangeType, string) {
	var diffs []string

	assetPorts := portSet(strings.Split(asset.OpenPorts, ","))
	scanPorts := portSet(r.OpenPorts)
	if !equalSet(assetPorts, scanPorts) {
		diffs = append(diffs, "开放端口: ["+strings.Join(sortedKeys(assetPorts), ",")+"] → ["+strings.Join(r.OpenPorts, ",")+"]")
	}
	if r.Hostname != "" && asset.Hostname != "" &&
		!strings.EqualFold(asset.Hostname, r.Hostname) &&
		!strings.Contains(strings.ToLower(asset.Hostname), strings.ToLower(r.Hostname)) &&
		!strings.Contains(strings.ToLower(r.Hostname), strings.ToLower(asset.Hostname)) {
		diffs = append(diffs, "主机名: "+asset.Hostname+" → "+r.Hostname)
	}
	if r.OS != "" && asset.OS != "" &&
		!strings.EqualFold(asset.OS, r.OS) &&
		!strings.Contains(strings.ToLower(asset.OS), strings.ToLower(r.OS)) &&
		!strings.Contains(strings.ToLower(r.OS), strings.ToLower(asset.OS)) {
		diffs = append(diffs, "操作系统: "+asset.OS+" → "+r.OS)
	}

	if len(diffs) > 0 {
		return model.DiscoveryChangeChanged, strings.Join(diffs, "; ")
	}
	if asset.OnlineStatus != model.AssetOnlineStatusOnline {
		return model.DiscoveryChangeOnline, "主机恢复在线"
	}
	return model.DiscoveryChangeNone, ""
}

// portSet 将端口字符串列表（"80/tcp" 或 "80"）归一化为端口号集合。
func portSet(ports []string) map[string]bool {
	out := map[string]bool{}
	for _, p := range ports {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		num := p
		if i := strings.IndexByte(p, '/'); i >= 0 {
			num = p[:i]
		}
		out[num] = true
	}
	return out
}

func equalSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func countChange(hosts []model.DiscoveredHost, t model.DiscoveryChangeType) int {
	n := 0
	for _, h := range hosts {
		if h.ChangeType == t {
			n++
		}
	}
	return n
}

// absentLastRound 判断该 IP 在指定规则的上一次成功运行中是否未出现（up）。
func absentLastRound(db *gorm.DB, ruleID uint, ip string, runID uint) (bool, error) {
	var prev model.DiscoveryRun
	err := db.Where("rule_id = ? AND status = ? AND id < ?",
		ruleID, model.DiscoveryRunStatusSuccess, runID).
		Order("id DESC").First(&prev).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil // 无上一轮记录，不满足“连续两轮”
	}
	if err != nil {
		return false, err
	}
	var count int64
	if err := db.Model(&model.DiscoveredHost{}).
		Where("run_id = ? AND ip = ? AND status = ?", prev.ID, ip, "up").
		Count(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}
