package httpapi

import (
	"encoding/csv"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"asset-registration-management-system/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type assetCSVColumn struct {
	Header string
	Value  func(model.Asset) string
	Apply  func(*model.Asset, string) error
}

var assetCSVColumns = []assetCSVColumn{
	{"序号", func(a model.Asset) string { return a.SequenceNo }, func(a *model.Asset, v string) error { a.SequenceNo = v; return nil }},
	{"IP地址", func(a model.Asset) string { return a.IP }, func(a *model.Asset, v string) error { a.IP = v; return nil }},
	{"主机名/设备名称", func(a model.Asset) string { return a.Hostname }, func(a *model.Asset, v string) error { a.Hostname = v; return nil }},
	{"MAC地址", func(a model.Asset) string { return a.MACAddress }, func(a *model.Asset, v string) error { a.MACAddress = v; return nil }},
	{"厂商", func(a model.Asset) string { return a.Manufacturer }, func(a *model.Asset, v string) error { a.Manufacturer = v; return nil }},
	{"资产类型", func(a model.Asset) string { return a.AssetType }, func(a *model.Asset, v string) error { a.AssetType = v; return nil }},
	{"操作系统", func(a model.Asset) string { return a.OS }, func(a *model.Asset, v string) error { a.OS = v; return nil }},
	{"开放端口", func(a model.Asset) string { return a.OpenPorts }, func(a *model.Asset, v string) error { a.OpenPorts = v; return nil }},
	{"运行服务/应用", func(a model.Asset) string { return a.RunningServices }, func(a *model.Asset, v string) error { a.RunningServices = v; a.BusinessSystem = v; return nil }},
	{"应用版本", func(a model.Asset) string { return a.AppVersion }, func(a *model.Asset, v string) error { a.AppVersion = v; return nil }},
	{"资产归属/负责人", func(a model.Asset) string { return a.Owner }, func(a *model.Asset, v string) error { a.Owner = v; return nil }},
	{"所在网段", func(a model.Asset) string { return a.Subnet }, func(a *model.Asset, v string) error { a.Subnet = v; return nil }},
	{"备注", func(a model.Asset) string { return a.Remark }, func(a *model.Asset, v string) error { a.Remark = v; return nil }},
}

var assetHeaderAliases = map[string]string{
	"assetNo":            "资产编号",
	"asset_no":           "资产编号",
	"资产编码":               "资产编号",
	"sequenceNo":         "序号",
	"序号":                 "序号",
	"IP地址":               "IP地址",
	"业务IP":               "IP地址",
	"ip":                 "IP地址",
	"主机名/设备名称":           "主机名/设备名称",
	"主机名":                "主机名/设备名称",
	"hostname":           "主机名/设备名称",
	"MAC地址":              "MAC地址",
	"macAddress":         "MAC地址",
	"mac":                "MAC地址",
	"assetType":          "资产类型",
	"managementIp":       "管理IP",
	"serialNo":           "序列号",
	"manufacturer":       "厂商",
	"model":              "型号",
	"location":           "机房",
	"rack":               "机柜",
	"rackPosition":       "机位",
	"os":                 "操作系统",
	"osVersion":          "系统版本",
	"cpu":                "CPU",
	"memory":             "内存",
	"disk":               "磁盘",
	"openPorts":          "开放端口",
	"runningServices":    "运行服务/应用",
	"appVersion":         "应用版本",
	"businessSystem":     "运行服务/应用",
	"environment":        "环境",
	"department":         "所属部门",
	"owner":              "资产归属/负责人",
	"subnet":             "所在网段",
	"maintenanceVendor":  "维保厂商",
	"purchaseDate":       "采购日期",
	"warrantyExpireDate": "维保到期日期",
	"status":             "状态",
	"onlineDate":         "上线日期",
	"remark":             "备注",
}

func (h *Handler) DownloadAssetImportTemplate(c *gin.Context) {
	if strings.EqualFold(c.Query("format"), "csv") {
		writeAssetCSV(c, "asset-import-template.csv", nil)
		return
	}
	writeAssetXLSX(c, "asset-import-template.xlsx", assetImportTemplateSheets())
}

func (h *Handler) ExportAssets(c *gin.Context) {
	var assets []model.Asset
	db := applyAssetSort(applyAssetFilters(h.db.Model(&model.Asset{}), c), c)
	if err := db.Find(&assets).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "导出资产失败")
		return
	}
	writeAssetCSV(c, "assets-export.csv", assets)
}

// ExportAssetStats 导出资产统计报表（CSV）：在线状态分布、资产类型分布、TOP 开放端口、TOP 服务。
// @Summary 资产统计报表导出
// @Description 导出资产统计汇总为 CSV（在线状态/类型/TOP 端口/TOP 服务）
// @Tags assets
// @Produce text/csv
// @Success 200 {string} string "CSV 文件"
// @Router /assets/stats/export [get]
// @Security BearerAuth
func (h *Handler) ExportAssetStats(c *gin.Context) {
	base := applyAssetFilters(h.db.Model(&model.Asset{}), c)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "导出统计失败")
		return
	}
	var assets []model.Asset
	if err := base.Select("open_ports", "running_services", "online_status").Find(&assets).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "导出统计失败")
		return
	}
	openPortValues := make([]string, 0, len(assets))
	serviceValues := make([]string, 0, len(assets))
	statusCounts := map[string]int{}
	for _, a := range assets {
		status := string(a.OnlineStatus)
		if status == "" {
			status = "unknown"
		}
		statusCounts[status]++
		if strings.TrimSpace(a.OpenPorts) != "" {
			openPortValues = append(openPortValues, a.OpenPorts)
		}
		if strings.TrimSpace(a.RunningServices) != "" {
			serviceValues = append(serviceValues, a.RunningServices)
		}
	}

	var buf strings.Builder
	writeCSVRow := func(row ...string) {
		for i, v := range row {
			if i > 0 {
				buf.WriteString(",")
			}
			buf.WriteString(csvEscape(v))
		}
		buf.WriteString("\n")
	}

	writeCSVRow("资产统计报表", "生成时间", time.Now().Format("2006-01-02 15:04:05"))
	writeCSVRow("资产总数", fmt.Sprintf("%d", total), "")
	writeCSVRow()
	writeCSVRow("在线状态分布")
	writeCSVRow("状态", "数量")
	for _, k := range []string{"online", "offline", "unknown"} {
		writeCSVRow(k, fmt.Sprintf("%d", statusCounts[k]))
	}
	writeCSVRow()
	writeCSVRow("资产类型分布")
	writeCSVRow("类型", "数量")
	for _, item := range assetGroupedCounts(base, "asset_type", 20) {
		writeCSVRow(item.Label, fmt.Sprintf("%d", item.Count))
	}
	writeCSVRow()
	writeCSVRow("TOP 开放端口")
	writeCSVRow("端口", "资产数")
	for _, item := range topAssetTokens(openPortValues, 10, true) {
		writeCSVRow(item.Label, fmt.Sprintf("%d", item.Count))
	}
	writeCSVRow()
	writeCSVRow("TOP 服务")
	writeCSVRow("服务", "资产数")
	for _, item := range topAssetTokens(serviceValues, 10, false) {
		writeCSVRow(item.Label, fmt.Sprintf("%d", item.Count))
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="asset-stats-report.csv"`)
	c.String(http.StatusOK, "\xEF\xBB\xBF"+buf.String())
}

// csvEscape CSV 单元格转义（含逗号/引号时加引号）。
func csvEscape(v string) string {
	if strings.ContainsAny(v, ",\"\n") {
		return `"` + strings.ReplaceAll(v, `"`, `""`) + `"`
	}
	return v
}

func (h *Handler) ImportAssets(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "请选择 CSV 文件")
		return
	}
	opened, err := file.Open()
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "打开导入文件失败")
		return
	}
	defer func() { _ = opened.Close() }()

	records, err := readAssetImportRows(file.Filename, opened, file.Size)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "资产清单解析失败: "+err.Error())
		return
	}
	records = normalizeAssetImportRecords(records)
	if len(records) < 2 {
		errorJSON(c, http.StatusBadRequest, "资产清单至少需要表头和一行资产数据")
		return
	}

	headerIndex := assetImportHeaderIndex(records[0])
	created := 0
	updated := 0
	var rowErrors []string
	txErr := h.db.Transaction(func(tx *gorm.DB) error {
		for i, row := range records[1:] {
			if csvRowEmpty(row) {
				continue
			}
			asset, err := assetFromCSVRow(headerIndex, row)
			if err != nil {
				rowErrors = append(rowErrors, fmt.Sprintf("第 %d 行: %s", i+2, err.Error()))
				continue
			}
			var existing model.Asset
			err = tx.Where("asset_no = ? OR ip = ?", asset.AssetNo, asset.IP).First(&existing).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := tx.Create(&asset).Error; err != nil {
					rowErrors = append(rowErrors, fmt.Sprintf("第 %d 行: %s", i+2, err.Error()))
					continue
				}
				created++
				continue
			}
			if err != nil {
				rowErrors = append(rowErrors, fmt.Sprintf("第 %d 行: %s", i+2, err.Error()))
				continue
			}
			asset.ID = existing.ID
			asset.CreatedAt = existing.CreatedAt
			if err := tx.Save(&asset).Error; err != nil {
				rowErrors = append(rowErrors, fmt.Sprintf("第 %d 行: %s", i+2, err.Error()))
				continue
			}
			updated++
		}
		if len(rowErrors) > 0 {
			return errors.New(strings.Join(rowErrors, "; "))
		}
		return nil
	})
	if txErr != nil {
		errorJSON(c, http.StatusBadRequest, "导入失败: "+txErr.Error())
		return
	}
	h.audit(currentUser(c).ID, "asset", 0, "import", fmt.Sprintf("created=%d updated=%d", created, updated))
	c.JSON(http.StatusOK, gin.H{"created": created, "updated": updated})
}

func readAssetImportRows(filename string, file multipart.File, size int64) ([][]string, error) {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".xlsx":
		return readXLSXRows(file, size)
	default:
		reader := csv.NewReader(file)
		reader.FieldsPerRecord = -1
		reader.TrimLeadingSpace = true
		return reader.ReadAll()
	}
}

func writeAssetCSV(c *gin.Context, filename string, assets []model.Asset) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	_, _ = c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(c.Writer)
	headers := make([]string, 0, len(assetCSVColumns))
	for _, col := range assetCSVColumns {
		headers = append(headers, col.Header)
	}
	_ = writer.Write(headers)
	for _, asset := range assets {
		row := make([]string, 0, len(assetCSVColumns))
		for _, col := range assetCSVColumns {
			row = append(row, col.Value(asset))
		}
		_ = writer.Write(row)
	}
	writer.Flush()
}

func assetImportTemplateSheets() []xlsxSheet {
	headers := make([]string, 0, len(assetCSVColumns))
	blankRow := make([]string, 0, len(assetCSVColumns))
	for _, col := range assetCSVColumns {
		headers = append(headers, col.Header)
		blankRow = append(blankRow, "")
	}
	return []xlsxSheet{
		{
			Name: "资产导入模板",
			Rows: [][]string{
				headers,
				blankRow,
			},
		},
		{
			Name: "填写说明",
			Rows: [][]string{
				{"项目", "说明"},
				{"必填字段", "IP地址；主机名/设备名称为空时系统会用 IP 自动填充"},
				{"内部编号", "系统会根据 IP地址 自动生成资产编号，用于重复导入时更新同一资产"},
				{"导入规则", "按自动生成的资产编号匹配，编号不存在则新增，编号已存在则更新"},
				{"首行标题", "支持原始测绘文件第一行是“全资产详细清单”、第二行才是表头的格式"},
				{"字段范围", "模板按测绘结果资产字段设计"},
			},
		},
	}
}

func assetImportHeaderIndex(headers []string) map[string]int {
	index := map[string]int{}
	for i, header := range headers {
		normalized := normalizeHeader(header)
		if canonical, ok := assetHeaderAliases[normalized]; ok {
			normalized = canonical
		}
		index[normalized] = i
	}
	return index
}

func normalizeAssetImportRecords(records [][]string) [][]string {
	for i, row := range records {
		headerIndex := assetImportHeaderIndex(row)
		if _, ok := headerIndex["IP地址"]; !ok {
			continue
		}
		if _, ok := headerIndex["主机名/设备名称"]; ok {
			return records[i:]
		}
	}
	return records
}

func assetFromCSVRow(headerIndex map[string]int, row []string) (model.Asset, error) {
	asset := model.Asset{Status: model.AssetStatusInUse}
	for _, col := range assetCSVColumns {
		idx, ok := headerIndex[col.Header]
		if !ok || idx >= len(row) {
			continue
		}
		value := cleanAssetPlaceholder(col.Header, row[idx])
		if err := col.Apply(&asset, value); err != nil {
			return model.Asset{}, fmt.Errorf("%s %s", col.Header, err.Error())
		}
	}
	if idx, ok := headerIndex["资产编号"]; ok && idx < len(row) {
		asset.AssetNo = strings.TrimSpace(row[idx])
	}
	asset.AssetNo = strings.TrimSpace(asset.AssetNo)
	asset.Hostname = strings.TrimSpace(asset.Hostname)
	asset.IP = strings.TrimSpace(asset.IP)
	if asset.IP == "" {
		return model.Asset{}, errors.New("IP地址不能为空")
	}
	if asset.AssetNo == "" {
		asset.AssetNo = assetNoFromIP(asset.IP)
	}
	if asset.Status == "" {
		asset.Status = model.AssetStatusInUse
	}
	return asset, nil
}

func assetNoFromIP(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("IP-")
	lastDash := false
	for _, r := range ip {
		if (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.TrimRight(builder.String(), "-")
}

func normalizeHeader(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "\ufeff")
}

func csvRowEmpty(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func normalizeAssetStatus(value string) model.AssetStatus {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "使用中", "in_use", "in use":
		return model.AssetStatusInUse
	case "待上线", "pending":
		return model.AssetStatusPending
	case "维护中", "maintenance":
		return model.AssetStatusMaintenance
	case "已退役", "retired":
		return model.AssetStatusRetired
	case "已下线", "已报废", "decommissioned", "decommission":
		return model.AssetStatusDecommission
	default:
		return model.AssetStatus(value)
	}
}

func applyDate(target **time.Time, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		*target = nil
		return nil
	}
	for _, layout := range []string{"2006-01-02", "2006/01/02", "2006.01.02", time.RFC3339} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			*target = &parsed
			return nil
		}
	}
	if serial, err := strconv.ParseFloat(value, 64); err == nil && serial > 20000 && serial < 80000 {
		parsed := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(serial))
		*target = &parsed
		return nil
	}
	return errors.New("日期格式应为 YYYY-MM-DD")
}

func formatDate(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("2006-01-02")
}
