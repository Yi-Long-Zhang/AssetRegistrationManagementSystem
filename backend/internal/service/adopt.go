package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"asset-registration-management-system/backend/internal/model"

	"gorm.io/gorm"
)

// assetNoFromIP 依据 IP 生成资产编号，与 httpapi 中的规则保持一致：
// "IP-" 前缀，仅保留字母数字，其余字符替换为单个横线。
func assetNoFromIP(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("IP-")
	lastDash := false
	for _, r := range ip {
		if (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// AdoptHosts 将运行结果中选中的 new 主机批量纳管为资产，并写快照与审计。
// 返回实际纳管数量。
func (s *DiscoveryService) AdoptHosts(_ context.Context, runID uint, hostIDs []uint, actorID uint) (int, error) {
	if len(hostIDs) == 0 {
		return 0, fmt.Errorf("未选择要纳管的主机")
	}
	var hosts []model.DiscoveredHost
	if err := s.DB.Where("run_id = ? AND id IN ? AND change_type = ?",
		runID, hostIDs, model.DiscoveryChangeNew).Find(&hosts).Error; err != nil {
		return 0, err
	}
	if len(hosts) == 0 {
		return 0, fmt.Errorf("该运行中没有可纳管的新主机")
	}

	adopted := 0
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		for i := range hosts {
			h := &hosts[i]
			now := time.Now()
			asset := model.Asset{
				AssetNo:         assetNoFromIP(h.IP),
				AssetType:       "server",
				Hostname:        h.Hostname,
				IP:              h.IP,
				OpenPorts:       h.OpenPorts,
				RunningServices: h.Services,
				OS:              h.OS,
				Status:          model.AssetStatusInUse,
				OnlineStatus:    model.AssetOnlineStatusOnline,
				OnlineDate:      &now,
				DiscoveredAt:    &now,
				LastSeenAt:      &now,
			}
			if err := tx.Create(&asset).Error; err != nil {
				return fmt.Errorf("创建资产 %s 失败: %w", h.IP, err)
			}
			snapper := AssetSnapshotter{DB: tx}
			if err := snapper.CreateSnapshot(&asset, model.SnapshotSourceDiscovery, &actorID, "create"); err != nil {
				return err
			}
			if err := tx.Create(&model.AuditLog{
				ActorID: actorID, Entity: "asset", EntityID: asset.ID, Action: "create",
				Detail: fmt.Sprintf("发现纳管主机 %s (%s)", h.IP, h.Hostname),
			}).Error; err != nil {
				return err
			}
			if err := tx.Model(h).Updates(map[string]any{
				"adopted": true, "matched_asset_id": asset.ID,
			}).Error; err != nil {
				return err
			}
			adopted++
		}
		return nil
	})
	return adopted, err
}

// ApplyHostChanges 将选中的 changed/online/offline 结果应用到资产台账，并写快照与审计。
// 返回实际应用数量。
func (s *DiscoveryService) ApplyHostChanges(_ context.Context, runID uint, hostIDs []uint, actorID uint) (int, error) {
	if len(hostIDs) == 0 {
		return 0, fmt.Errorf("未选择要应用的主机")
	}
	var hosts []model.DiscoveredHost
	if err := s.DB.Where("run_id = ? AND id IN ? AND change_type IN ?", runID, hostIDs,
		[]model.DiscoveryChangeType{model.DiscoveryChangeChanged, model.DiscoveryChangeOnline, model.DiscoveryChangeOffline},
	).Find(&hosts).Error; err != nil {
		return 0, err
	}
	if len(hosts) == 0 {
		return 0, fmt.Errorf("该运行中没有可应用的变更结果")
	}

	applied := 0
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		for i := range hosts {
			h := &hosts[i]
			if h.MatchedAssetID == nil {
				continue
			}
			var asset model.Asset
			if err := tx.First(&asset, *h.MatchedAssetID).Error; err != nil {
				return err
			}
			now := time.Now()
			changeType := "update"
			switch h.ChangeType {
			case model.DiscoveryChangeOffline:
				asset.OnlineStatus = model.AssetOnlineStatusOffline
				changeType = "offline"
			default:
				if h.Hostname != "" {
					asset.Hostname = h.Hostname
				}
				if h.OpenPorts != "" {
					asset.OpenPorts = h.OpenPorts
				}
				if h.Services != "" {
					asset.RunningServices = h.Services
				}
				if h.OS != "" {
					asset.OS = h.OS
				}
				asset.OnlineStatus = model.AssetOnlineStatusOnline
				if asset.DiscoveredAt == nil {
					asset.DiscoveredAt = &now
				}
			}
			asset.LastSeenAt = &now
			if err := tx.Save(&asset).Error; err != nil {
				return err
			}
			snapper := AssetSnapshotter{DB: tx}
			if err := snapper.CreateSnapshot(&asset, model.SnapshotSourceDiscovery, &actorID, changeType); err != nil {
				return err
			}
			if err := tx.Create(&model.AuditLog{
				ActorID: actorID, Entity: "asset", EntityID: asset.ID, Action: changeType,
				Detail: fmt.Sprintf("发现结果应用: %s (%s)", h.IP, h.ChangeType),
			}).Error; err != nil {
				return err
			}
			if err := tx.Model(h).Update("applied", true).Error; err != nil {
				return err
			}
			applied++
		}
		return nil
	})
	return applied, err
}
