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
// - 资产此前在线、本轮未出现、LastSeenAt 超过离线窗口且最近一次任意规则运行仍缺席 → offline
// 同时更新运行记录统计。
func (s *DiscoveryService) reconcile(run *model.DiscoveryRun, results []ScanResult, now time.Time) error {
	var assets []model.Asset
	if err := s.DB.Find(&assets).Error; err != nil {
		return err
	}
	byIP := make(map[string]*model.Asset, len(assets))
	byMAC := make(map[string]*model.Asset, len(assets))
	for i := range assets {
		a := &assets[i]
		for _, ip := range assetIPs(a) {
			byIP[ip] = a
		}
		if a.MACAddress != "" {
			byMAC[normalizeMAC(a.MACAddress)] = a
		}
	}

	seen := make(map[string]bool, len(results))
	markSeen := func(a *model.Asset) {
		for _, ip := range assetIPs(a) {
			seen[ip] = true
		}
	}
	hosts := make([]model.DiscoveredHost, 0, len(results))
	for _, r := range results {
		if r.Status != "up" {
			continue
		}
		seen[r.IP] = true
		change := model.DiscoveryChangeNew
		risk := model.ChangeRiskLow
		var matchedID *uint
		var diff string
		// 匹配顺序：MAC（同网段 ARP 最可靠）→ IP（含管理 IP 与关联 IP）
		var asset *model.Asset
		if r.MAC != "" {
			asset = byMAC[normalizeMAC(r.MAC)]
		}
		if asset == nil {
			asset = byIP[r.IP]
		}
		if asset != nil {
			matchedID = &asset.ID
			change, diff, risk = classifyChange(asset, r)
			markSeen(asset) // 多网卡/多 IP 资产：其余 IP 一并标记，防止误判离线
		}
		hosts = append(hosts, model.DiscoveredHost{
			RunID:          run.ID,
			IP:             r.IP,
			MAC:            r.MAC,
			Hostname:       r.Hostname,
			Status:         r.Status,
			OpenPorts:      strings.Join(r.OpenPorts, ","),
			Services:       joinLines(r.Services),
			OS:             r.OS,
			ChangeType:     change,
			ChangeRisk:     risk,
			MatchedAssetID: matchedID,
			DiffSummary:    diff,
		})
	}

	// 离线判定（P4）：时间窗口 + 跨规则确认
	// - 时间窗口：资产 LastSeenAt 距今超过 offline_after_hours 才可能判离线（替代连续轮数判据）
	// - 跨规则确认：最近一次任意规则的成功运行中该 IP 同样缺席，避免“另一网段仍在线”误判
	var offline []model.DiscoveredHost
	offlineAfter := time.Duration(s.Config.Discovery.OfflineAfterHours) * time.Hour
	if offlineAfter <= 0 {
		offlineAfter = 24 * time.Hour // 兜底默认
	}
	for _, asset := range byIP {
		if assetSeen(asset, seen) {
			continue
		}
		if asset.OnlineStatus != model.AssetOnlineStatusOnline && asset.OnlineStatus != model.AssetOnlineStatusUnknown {
			continue // 未纳管/已退休等不参与离线判定
		}
		if asset.LastSeenAt == nil {
			continue // 从未被扫描发现过，无从判定离线
		}
		if time.Since(*asset.LastSeenAt) < offlineAfter {
			continue // 时间窗口内未响应，暂不判离线
		}
		absentLatest, err := absentInLatestRun(s.DB, assetIPs(asset)[0], run.ID)
		if err != nil {
			return err
		}
		if !absentLatest {
			continue // 最近一次任意规则运行仍发现该主机（如其它网段在线）
		}
		offline = append(offline, model.DiscoveredHost{
			RunID:          run.ID,
			IP:             assetIPs(asset)[0],
			Status:         "down",
			ChangeType:     model.DiscoveryChangeOffline,
			ChangeRisk:     model.ChangeRiskHigh, // 离线属于高风险变更，需人工确认
			MatchedAssetID: &asset.ID,
			DiffSummary:    "超过离线窗口未发现主机（跨规则确认）",
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
	// 回填内存态，保证调用方（含单测）可读
	run.Hosts = append(run.Hosts, hosts...)
	run.Hosts = append(run.Hosts, offline...)

	run.TotalHosts = len(hosts)
	run.NewCount = countChange(hosts, model.DiscoveryChangeNew)
	run.ChangedCount = countChange(hosts, model.DiscoveryChangeChanged)
	run.OnlineCount = countChange(hosts, model.DiscoveryChangeOnline)
	run.OfflineCount = len(offline)
	return nil
}

// classifyChange 对比资产当前字段与扫描结果，返回变更类型、diff 摘要与风险级别。
// 风险分级：仅端口新增为 low（可自动应用）；端口关闭/主机名/OS/服务变化为 high（需人工确认）。
func classifyChange(asset *model.Asset, r ScanResult) (model.DiscoveryChangeType, string, model.ChangeRiskLevel) {
	var diffs []string
	risk := model.ChangeRiskLow

	assetPorts := portSet(strings.Split(asset.OpenPorts, ","))
	scanPorts := portSet(r.OpenPorts)
	if !equalSet(assetPorts, scanPorts) {
		added := diffKeys(scanPorts, assetPorts)   // 新增端口
		removed := diffKeys(assetPorts, scanPorts) // 关闭端口
		var parts []string
		if len(added) > 0 {
			parts = append(parts, "新增 "+strings.Join(added, ","))
		}
		if len(removed) > 0 {
			parts = append(parts, "关闭 "+strings.Join(removed, ","))
		}
		diffs = append(diffs, "开放端口: "+strings.Join(parts, "; "))
		if len(removed) > 0 {
			risk = model.ChangeRiskHigh // 端口关闭属于高风险变更
		}
	}
	if r.Hostname != "" && asset.Hostname != "" &&
		!strings.EqualFold(asset.Hostname, r.Hostname) &&
		!strings.Contains(strings.ToLower(asset.Hostname), strings.ToLower(r.Hostname)) &&
		!strings.Contains(strings.ToLower(r.Hostname), strings.ToLower(asset.Hostname)) {
		diffs = append(diffs, "主机名: "+asset.Hostname+" → "+r.Hostname)
		risk = model.ChangeRiskHigh
	}
	if r.OS != "" && asset.OS != "" &&
		!strings.EqualFold(asset.OS, r.OS) &&
		!strings.Contains(strings.ToLower(asset.OS), strings.ToLower(r.OS)) &&
		!strings.Contains(strings.ToLower(r.OS), strings.ToLower(asset.OS)) {
		diffs = append(diffs, "操作系统: "+asset.OS+" → "+r.OS)
		risk = model.ChangeRiskHigh
	}
	if svc := serviceDiff(strings.Split(asset.RunningServices, "\n"), r.Services); svc != "" {
		diffs = append(diffs, "服务: "+svc)
		risk = model.ChangeRiskHigh
	}

	if len(diffs) > 0 {
		return model.DiscoveryChangeChanged, strings.Join(diffs, "; "), risk
	}
	if asset.OnlineStatus != model.AssetOnlineStatusOnline {
		return model.DiscoveryChangeOnline, "主机恢复在线", model.ChangeRiskLow
	}
	return model.DiscoveryChangeNone, "", model.ChangeRiskLow
}

// diffKeys 返回 map a 中存在但 b 中不存在的键（排序），用于端口新增/关闭明细。
func diffKeys(a, b map[string]bool) []string {
	var out []string
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// serviceDiff 按端口对齐比较资产与扫描的服务信息；同一端口服务描述变化即视为服务变更。
func serviceDiff(assetServices, scanServices []string) string {
	assetMap := serviceMap(assetServices)
	scanMap := serviceMap(scanServices)
	var diffs []string
	for port, info := range scanMap {
		old, ok := assetMap[port]
		if ok && old != "" && info != "" && !strings.EqualFold(old, info) {
			diffs = append(diffs, port+"("+old+" → "+info+")")
		}
	}
	return strings.Join(diffs, ", ")
}

// serviceMap 将服务行（"80/tcp: http Apache 2.4" 或 "80/tcp: http"）解析为 端口号 -> 服务描述。
func serviceMap(rows []string) map[string]string {
	out := map[string]string{}
	for _, row := range rows {
		row = strings.TrimSpace(row)
		if row == "" {
			continue
		}
		portPart, info := row, ""
		if idx := strings.Index(row, ":"); idx >= 0 {
			portPart = strings.TrimSpace(row[:idx])
			info = strings.TrimSpace(row[idx+1:])
		}
		out[strings.Split(portPart, "/")[0]] = info
	}
	return out
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

// assetIPs 返回资产的全部关联 IP（主 IP + 管理 IP + 附加关联 IP），去空去重。
func assetIPs(a *model.Asset) []string {
	var ips []string
	seen := map[string]bool{}
	add := func(ip string) {
		ip = strings.TrimSpace(ip)
		if ip == "" || seen[ip] {
			return
		}
		seen[ip] = true
		ips = append(ips, ip)
	}
	add(a.IP)
	add(a.ManagementIP)
	for _, ip := range strings.Split(a.AdditionalIPs, ",") {
		add(ip)
	}
	return ips
}

// assetSeen 判断资产任一关联 IP 是否在本轮 seen 集合中。
func assetSeen(a *model.Asset, seen map[string]bool) bool {
	for _, ip := range assetIPs(a) {
		if seen[ip] {
			return true
		}
	}
	return false
}

// normalizeMAC 归一化 MAC：小写、去分隔符（: - .）。
func normalizeMAC(mac string) string {
	mac = strings.ToLower(strings.TrimSpace(mac))
	var b strings.Builder
	for _, r := range mac {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// absentInLatestRun 判断该 IP 在最近一次任意规则的成功运行中是否未出现（up）。
// 跨规则确认：只要任一规则最近一轮仍发现该主机，就不判离线（防止"另一网段仍在线"误判）。
func absentInLatestRun(db *gorm.DB, ip string, excludeRunID uint) (bool, error) {
	var prev model.DiscoveryRun
	err := db.Where("status = ? AND id != ?", model.DiscoveryRunStatusSuccess, excludeRunID).
		Order("id DESC").First(&prev).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true, nil // 无任何历史成功运行，视为跨规则确认通过（等待时间窗口累计）
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
