package httpapi

import (
	"sort"
	"strconv"
	"strings"

	"asset-registration-management-system/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type assetBucket struct {
	Label string `json:"label"`
	Count int64  `json:"count"`
}

var assetSortColumns = map[string]string{
	"sequenceNo":      "CAST(sequence_no AS INTEGER)",
	"ip":              "ip",
	"hostname":        "hostname",
	"macAddress":      "mac_address",
	"manufacturer":    "manufacturer",
	"assetType":       "asset_type",
	"os":              "os",
	"openPorts":       "open_ports",
	"runningServices": "running_services",
	"appVersion":      "app_version",
	"owner":           "owner",
	"subnet":          "subnet",
	"createdAt":       "created_at",
	"updatedAt":       "updated_at",
}

func applyAssetFilters(db *gorm.DB, c *gin.Context) *gorm.DB {
	q := strings.TrimSpace(c.Query("q"))
	if q != "" {
		like := "%" + q + "%"
		db = db.Where("asset_no LIKE ? OR sequence_no LIKE ? OR hostname LIKE ? OR ip LIKE ? OR mac_address LIKE ? OR manufacturer LIKE ? OR asset_type LIKE ? OR os LIKE ? OR open_ports LIKE ? OR running_services LIKE ? OR app_version LIKE ? OR owner LIKE ? OR subnet LIKE ? OR remark LIKE ?", like, like, like, like, like, like, like, like, like, like, like, like, like, like)
	}
	if value := strings.TrimSpace(c.Query("assetType")); value != "" {
		db = db.Where("asset_type = ?", value)
	}
	if value := strings.TrimSpace(c.Query("subnet")); value != "" {
		db = db.Where("subnet = ?", value)
	}
	if value := strings.TrimSpace(c.Query("owner")); value != "" {
		db = db.Where("owner = ?", value)
	}
	if value := strings.TrimSpace(c.Query("rack")); value != "" {
		db = db.Where("rack = ?", value)
	}
	if value := strings.TrimSpace(c.Query("location")); value != "" {
		db = db.Where("location = ?", value)
	}
	if value := strings.TrimSpace(c.Query("manufacturer")); value != "" {
		db = db.Where("manufacturer = ?", value)
	}
	if value := strings.TrimSpace(c.Query("openPort")); value != "" {
		db = db.Where("open_ports LIKE ?", "%"+value+"%")
	}
	if value := strings.TrimSpace(c.Query("service")); value != "" {
		db = db.Where("running_services LIKE ?", "%"+value+"%")
	}
	if value := strings.TrimSpace(c.Query("onlineStatus")); value != "" {
		db = db.Where("online_status = ?", value)
	}
	return db
}

func applyAssetSort(db *gorm.DB, c *gin.Context) *gorm.DB {
	column := assetSortColumns[strings.TrimSpace(c.Query("sortBy"))]
	if column == "" {
		column = "id"
	}
	direction := "desc"
	if strings.EqualFold(c.Query("sortOrder"), "asc") {
		direction = "asc"
	}
	return db.Order(column + " " + direction).Order("id desc")
}

func assetPagination(c *gin.Context) (int, int) {
	page := boundedQueryInt(c.Query("page"), 1, 1, 1000000)
	pageSize := boundedQueryInt(c.Query("pageSize"), 20, 1, 200)
	return page, pageSize
}

func boundedQueryInt(value string, fallback, min, max int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	if parsed < min {
		return min
	}
	if parsed > max {
		return max
	}
	return parsed
}

func assetGroupedCounts(db *gorm.DB, column string, limit int) []assetBucket {
	var buckets []assetBucket
	db.Select(column + " AS label, COUNT(*) AS count").
		Where(column + " <> ''").
		Group(column).
		Order("count desc").
		Limit(limit).
		Scan(&buckets)
	return buckets
}

func topAssetTokens(values []string, limit int, splitPorts bool) []assetBucket {
	counts := map[string]int64{}
	for _, value := range values {
		for _, item := range splitAssetValue(value, splitPorts) {
			counts[item]++
		}
	}
	buckets := make([]assetBucket, 0, len(counts))
	for label, count := range counts {
		buckets = append(buckets, assetBucket{Label: label, Count: count})
	}
	sort.Slice(buckets, func(i, j int) bool {
		if buckets[i].Count == buckets[j].Count {
			return buckets[i].Label < buckets[j].Label
		}
		return buckets[i].Count > buckets[j].Count
	})
	if len(buckets) > limit {
		return buckets[:limit]
	}
	return buckets
}

func splitAssetValue(value string, splitPorts bool) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	replacer := strings.NewReplacer("；", ";", "，", ",", "\n", ";")
	value = replacer.Replace(value)
	separators := func(r rune) bool {
		if splitPorts {
			return r == ',' || r == ';' || r == ' '
		}
		return r == ';'
	}
	parts := strings.FieldsFunc(value, separators)
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		items = append(items, part)
	}
	return items
}

func assetPlaceholderCleanable(header string) bool {
	switch header {
	case "序号", "IP地址", "资产编号", "资产类型", "状态":
		return false
	default:
		return true
	}
}

func cleanAssetPlaceholder(header, value string) string {
	value = strings.TrimSpace(value)
	if assetPlaceholderCleanable(header) && value == "17" {
		return ""
	}
	return value
}

func cleanExistingAssetPlaceholders(db *gorm.DB) error {
	fields := []string{
		"hostname", "mac_address", "manufacturer", "os", "open_ports", "running_services",
		"business_system", "app_version", "owner", "subnet", "remark", "management_ip",
		"serial_no", "model", "location", "rack", "rack_position", "os_version", "cpu",
		"memory", "disk", "environment", "department", "maintenance_vendor",
	}
	for _, field := range fields {
		if err := db.Model(&model.Asset{}).Where("TRIM(COALESCE("+field+", '')) = ?", "17").Update(field, "").Error; err != nil {
			return err
		}
	}
	return nil
}
