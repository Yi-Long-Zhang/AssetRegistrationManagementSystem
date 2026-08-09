package service

import (
	"encoding/json"
	"sort"
	"strings"

	"asset-registration-management-system/backend/internal/model"

	"gorm.io/gorm"
)

// 资产类型标准值（与前端字典保持一致）。
const (
	AssetTypeServer      = "server"      // 服务器
	AssetTypeDatabase    = "database"    // 数据库
	AssetTypeNetwork     = "network"     // 网络设备
	AssetTypeWorkstation = "workstation" // 工作站
)

// 数据库特征端口
var dbPorts = map[string]bool{
	"3306":  true, // MySQL
	"5432":  true, // PostgreSQL
	"1521":  true, // Oracle
	"6379":  true, // Redis
	"27017": true, // MongoDB
	"1433":  true, // MSSQL
	"9200":  true, // Elasticsearch
	"9042":  true, // Cassandra
}

// 网络设备特征端口
var networkPorts = map[string]bool{
	"23":  true, // Telnet
	"161": true, // SNMP
	"162": true, // SNMP trap
}

// 工作站特征端口
var workstationPorts = map[string]bool{
	"3389": true, // RDP
	"5900": true, // VNC
	"5901": true,
}

// 网络设备服务/OS 关键词
var networkKeywords = []string{
	"router", "switch", "firewall", "fortigate", "paloalto", "pan-os",
	"cisco", "huawei", "h3c", "junos", "nsx", "vrp", "nx-os", "ios",
}

// 工作站 OS 关键词
var workstationOSKeywords = []string{
	"windows 10", "windows 11", "windows 7", "windows 8", "macos",
}

// inferAssetType 根据开放端口、服务与操作系统推断资产类型。
// 返回 "" 表示无法推断（不设置 assetType）。优先级：数据库 > 网络设备 > 工作站 > 服务器。
func inferAssetType(ports []string, services []string, os string) string {
	portNums := extractPortNumbers(ports)
	lowerOS := strings.ToLower(os)
	lowerServices := strings.ToLower(strings.Join(services, " "))

	// 1. 数据库
	for p := range portNums {
		if dbPorts[p] {
			return AssetTypeDatabase
		}
	}
	// 2. 网络设备
	for p := range portNums {
		if networkPorts[p] {
			return AssetTypeNetwork
		}
	}
	for _, kw := range networkKeywords {
		if strings.Contains(lowerServices, kw) || strings.Contains(lowerOS, kw) {
			return AssetTypeNetwork
		}
	}
	// 3. 工作站
	for p := range portNums {
		if workstationPorts[p] {
			return AssetTypeWorkstation
		}
	}
	for _, kw := range workstationOSKeywords {
		if strings.Contains(lowerOS, kw) {
			return AssetTypeWorkstation
		}
	}
	// 4. 服务器：存在开放端口即视为服务器
	if len(portNums) > 0 {
		return AssetTypeServer
	}
	return ""
}

// extractPortNumbers 将 "80/tcp"、"443" 等端口项提取为端口号集合（去重）。
func extractPortNumbers(ports []string) map[string]bool {
	out := map[string]bool{}
	for _, p := range ports {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		port := strings.Split(p, "/")[0]
		if port != "" {
			out[port] = true
		}
	}
	return out
}

// 高危端口清单：新开放这些端口时产生异常端口预警
var highRiskPorts = map[string]bool{
	"22":    true, // SSH（弱口令风险）
	"23":    true, // Telnet 明文
	"3389":  true, // RDP
	"445":   true, // SMB
	"3306":  true, // MySQL 暴露
	"5432":  true, // PostgreSQL 暴露
	"6379":  true, // Redis 未授权
	"27017": true, // MongoDB 未授权
	"9200":  true, // Elasticsearch 未授权
	"11211": true, // Memcached
	"8080":  true, // 管理后台常见端口
	"9090":  true, // Prometheus/Grafana 等
}

// detectSuspiciousPorts 对比资产历史端口基线，返回新开放的端口清单（按端口号排序）。
// historical 为历史快照中的端口列表（可为 nil，表示无基线）。
func detectSuspiciousPorts(current, historical []string) []string {
	cur := extractPortNumbers(current)
	old := extractPortNumbers(historical)
	var added []string
	for p := range cur {
		if !old[p] {
			added = append(added, p)
		}
	}
	sort.Strings(added)
	return added
}

// detectHighRiskPorts 从新增端口清单中过滤出高危端口。
func detectHighRiskPorts(added []string) []string {
	var out []string
	for _, p := range added {
		if highRiskPorts[p] {
			out = append(out, p)
		}
	}
	return out
}

// lastSnapshotPorts 返回资产最近一次快照中的开放端口列表（作端口基线），无快照时返回 nil。
func lastSnapshotPorts(db *gorm.DB, assetID uint) []string {
	var last model.AssetSnapshot
	err := db.Where("asset_id = ?", assetID).Order("id DESC").First(&last).Error
	if err != nil {
		return nil
	}
	var m map[string]string
	if json.Unmarshal([]byte(last.SnapshotJSON), &m) != nil {
		return nil
	}
	return strings.Split(m["openPorts"], ",")
}

// lastUpHosts 返回规则上次成功运行中发现的存活主机 IP 列表（增量扫描目标）。
func lastUpHosts(db *gorm.DB, ruleID uint) []string {
	var run model.DiscoveryRun
	err := db.Where("rule_id = ? AND status = ?", ruleID, model.DiscoveryRunStatusSuccess).
		Order("id DESC").First(&run).Error
	if err != nil {
		return nil
	}
	var hosts []model.DiscoveredHost
	if err := db.Where("run_id = ? AND status = ?", run.ID, "up").Find(&hosts).Error; err != nil {
		return nil
	}
	seen := map[string]bool{}
	var ips []string
	for _, h := range hosts {
		if h.IP != "" && !seen[h.IP] {
			seen[h.IP] = true
			ips = append(ips, h.IP)
		}
	}
	return ips
}
