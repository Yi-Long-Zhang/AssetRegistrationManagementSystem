package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"asset-registration-management-system/backend/internal/model"

	"gorm.io/gorm"
)

// snapshotFields 纳入资产快照与字段级 diff 的字段（对应 Asset 的 json tag）。
var snapshotFields = []string{
	"assetNo", "sequenceNo", "assetType", "hostname", "ip", "macAddress", "managementIp",
	"serialNo", "manufacturer", "model", "location", "rack", "rackPosition", "os", "osVersion",
	"cpu", "memory", "disk", "openPorts", "runningServices", "appVersion", "subnet",
	"businessSystem", "environment", "department", "owner", "maintenanceVendor",
	"status", "onlineStatus", "remark",
}

// AssetSnapshotter 负责资产快照的生成与字段级 diff 落库。
type AssetSnapshotter struct {
	DB *gorm.DB
}

// CreateSnapshot 为资产生成一份字段快照，与最近一份快照对比后落库 AssetSnapshot。
// changeType 由调用方传入（create/update/offline/online）。
func (s *AssetSnapshotter) CreateSnapshot(asset *model.Asset, source model.SnapshotSource, actorID *uint, changeType string) error {
	cur := assetFieldMap(asset)
	raw, err := json.Marshal(cur)
	if err != nil {
		return err
	}

	var last model.AssetSnapshot
	err = s.DB.Where("asset_id = ?", asset.ID).Order("id DESC").First(&last).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	var diffJSON, diffSummary string
	if err == nil {
		var prev map[string]string
		if json.Unmarshal([]byte(last.SnapshotJSON), &prev) == nil {
			if lines, summary := diffFields(prev, cur); len(lines) > 0 {
				if b, err := json.Marshal(lines); err == nil {
					diffJSON = string(b)
				}
				diffSummary = summary
			}
		}
	}

	snap := model.AssetSnapshot{
		AssetID:      asset.ID,
		Source:       source,
		ChangeType:   changeType,
		SnapshotJSON: string(raw),
		DiffJSON:     diffJSON,
		DiffSummary:  diffSummary,
		CreatedBy:    actorID,
		CreatedAt:    time.Now(),
	}
	return s.DB.Create(&snap).Error
}

// assetFieldMap 提取资产的快照字段为 map[string]string。
func assetFieldMap(asset *model.Asset) map[string]string {
	raw, _ := json.Marshal(asset)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	out := make(map[string]string, len(snapshotFields))
	for _, f := range snapshotFields {
		v, ok := m[f]
		if !ok || v == nil {
			out[f] = ""
			continue
		}
		if s, ok := v.(string); ok {
			out[f] = s
			continue
		}
		if b, err := json.Marshal(v); err == nil {
			out[f] = string(b)
		} else {
			out[f] = fmt.Sprint(v)
		}
	}
	return out
}

// diffFields 计算两个快照的字段级差异，返回可读行列表与摘要文本。
func diffFields(prev, cur map[string]string) ([]string, string) {
	var lines []string
	for _, f := range snapshotFields {
		ov, nv := prev[f], cur[f]
		if ov == nv {
			continue
		}
		switch {
		case ov == "" && nv != "":
			lines = append(lines, fmt.Sprintf("%s: (新增) → %s", f, nv))
		case nv == "":
			lines = append(lines, fmt.Sprintf("%s: %s → (清空)", f, ov))
		default:
			lines = append(lines, fmt.Sprintf("%s: %s → %s", f, ov, nv))
		}
	}
	return lines, strings.Join(lines, "\n")
}
