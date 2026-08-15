package httpapi

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"asset-registration-management-system/backend/internal/model"

	"github.com/gin-gonic/gin"
)

// ---------- IP 工具函数 ----------

func ipToUint(ip string) (uint32, bool) {
	parts := strings.Split(strings.TrimSpace(ip), ".")
	if len(parts) != 4 {
		return 0, false
	}
	var value uint32
	for _, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 || n > 255 {
			return 0, false
		}
		value = value<<8 | uint32(n)
	}
	return value, true
}

// cidrRange 解析 CIDR 得到起始/结束 IP（uint32）与可用地址总数。
// /31、/32 全部地址可用（RFC 3021），其余排除网络与广播地址。
func cidrRange(cidr string) (start, end uint32, total uint32, ok bool) {
	_, ipnet, err := net.ParseCIDR(strings.TrimSpace(cidr))
	if err != nil {
		return 0, 0, 0, false
	}
	ones, bits := ipnet.Mask.Size()
	s, ok := ipToUint(ipnet.IP.String())
	if !ok {
		return 0, 0, 0, false
	}
	hostBits := bits - ones
	size := uint32(1) << uint(hostBits)
	end = s + size - 1
	if ones >= 31 {
		total = size
	} else {
		total = size - 2
	}
	return s, end, total, true
}

// ---------- CRUD ----------

func (h *Handler) ListIPSegments(c *gin.Context) {
	var items []model.IPSegment
	if err := h.db.Order("id desc").Find(&items).Error; err != nil {
		errorJSON(c, 500, "查询 IP 网段失败")
		return
	}
	c.JSON(200, gin.H{"items": items})
}

func (h *Handler) CreateIPSegment(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		CIDR        string `json:"cidr" binding:"required"`
		Description string `json:"description"`
	}
	if !bind(c, &req) {
		return
	}
	if _, _, _, ok := cidrRange(req.CIDR); !ok {
		errorJSON(c, 400, "CIDR 格式不合法，示例：10.0.0.0/24")
		return
	}
	item := model.IPSegment{Name: strings.TrimSpace(req.Name), CIDR: strings.TrimSpace(req.CIDR), Description: req.Description}
	if err := h.db.Create(&item).Error; err != nil {
		log.Printf("create ip segment: %v", err)
		errorJSON(c, 400, "创建网段失败")
		return
	}
	h.audit(currentUser(c).ID, "ip_segment", item.ID, "create", item.CIDR)
	c.JSON(201, item)
}

func (h *Handler) UpdateIPSegment(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var item model.IPSegment
	if err := h.db.First(&item, id).Error; err != nil {
		statusForDBError(c, err, "网段不存在")
		return
	}
	var req struct {
		Name        string `json:"name" binding:"required"`
		CIDR        string `json:"cidr" binding:"required"`
		Description string `json:"description"`
	}
	if !bind(c, &req) {
		return
	}
	if _, _, _, ok := cidrRange(req.CIDR); !ok {
		errorJSON(c, 400, "CIDR 格式不合法，示例：10.0.0.0/24")
		return
	}
	item.Name = strings.TrimSpace(req.Name)
	item.CIDR = strings.TrimSpace(req.CIDR)
	item.Description = req.Description
	if err := h.db.Save(&item).Error; err != nil {
		log.Printf("update ip segment: %v", err)
		errorJSON(c, 400, "更新网段失败")
		return
	}
	h.audit(currentUser(c).ID, "ip_segment", item.ID, "update", item.CIDR)
	c.JSON(200, item)
}

func (h *Handler) DeleteIPSegment(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var item model.IPSegment
	if err := h.db.First(&item, id).Error; err != nil {
		statusForDBError(c, err, "网段不存在")
		return
	}
	if err := h.db.Delete(&item).Error; err != nil {
		log.Printf("delete ip segment: %v", err)
		errorJSON(c, 400, "删除网段失败")
		return
	}
	h.audit(currentUser(c).ID, "ip_segment", item.ID, "delete", item.CIDR)
	c.JSON(200, gin.H{"deleted": true})
}

// ---------- 使用情况与冲突 ----------

type ipUsageItem struct {
	IP       string `json:"ip"`
	AssetNo  string `json:"assetNo"`
	Hostname string `json:"hostname"`
}

type ipConflict struct {
	IP     string   `json:"ip"`
	Assets []string `json:"assets"`
	Count  int      `json:"count"`
}

// GetIPSegmentUsage 网段使用情况：总地址/已用/可用，已用 IP 清单与占用冲突。
func (h *Handler) GetIPSegmentUsage(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var seg model.IPSegment
	if err := h.db.First(&seg, id).Error; err != nil {
		statusForDBError(c, err, "网段不存在")
		return
	}
	start, end, total, valid := cidrRange(seg.CIDR)
	if !valid {
		errorJSON(c, 400, "网段 CIDR 不合法")
		return
	}
	var assets []model.Asset
	if err := h.db.Select("id", "asset_no", "hostname", "ip", "additional_ips").Find(&assets).Error; err != nil {
		errorJSON(c, 500, "查询资产失败")
		return
	}
	used := map[uint32][]string{} // ip → 资产编号列表（可能多台，即冲突）
	var usedIPs []ipUsageItem
	for _, asset := range assets {
		ips := []string{asset.IP}
		if asset.AdditionalIPs != "" {
			ips = append(ips, splitAssetValue(asset.AdditionalIPs, false)...)
		}
		for _, ip := range ips {
			value, ok := ipToUint(ip)
			if !ok || value < start || value > end {
				continue
			}
			used[value] = append(used[value], asset.AssetNo)
			usedIPs = append(usedIPs, ipUsageItem{IP: ip, AssetNo: asset.AssetNo, Hostname: asset.Hostname})
		}
	}
	var conflicts []ipConflict
	for ip, assetNos := range used {
		if len(assetNos) > 1 {
			conflicts = append(conflicts, ipConflict{IP: uintToIP(ip), Assets: assetNos, Count: len(assetNos)})
		}
	}
	c.JSON(200, gin.H{
		"segment":   seg,
		"total":     total,
		"used":      len(used),
		"available": int64(total) - int64(len(used)),
		"usedIPs":   usedIPs,
		"conflicts": conflicts,
	})
}

func uintToIP(value uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d", value>>24&255, value>>16&255, value>>8&255, value&255)
}

// checkIPConflict 资产写入后检查池内占用冲突（仅记录审计提示，不阻断）。
func (h *Handler) checkIPConflict(actorID uint, asset *model.Asset) {
	var segments []model.IPSegment
	if err := h.db.Find(&segments).Error; err != nil || len(segments) == 0 {
		return
	}
	ips := []string{asset.IP}
	if asset.AdditionalIPs != "" {
		ips = append(ips, splitAssetValue(asset.AdditionalIPs, false)...)
	}
	for _, seg := range segments {
		start, end, _, ok := cidrRange(seg.CIDR)
		if !ok {
			continue
		}
		for _, ip := range ips {
			value, ok := ipToUint(ip)
			if !ok || value < start || value > end {
				continue
			}
			var count int64
			_ = h.db.Model(&model.Asset{}).Where("id != ? AND (ip = ? OR additional_ips LIKE ?)", asset.ID, ip, "%"+ip+"%").Count(&count).Error
			if count > 0 {
				h.audit(actorID, "asset", asset.ID, "ip_conflict",
					fmt.Sprintf("IP %s 位于网段 %s(%s) 内，已被 %d 台其他资产占用", ip, seg.Name, seg.CIDR, count))
			}
		}
	}
}
