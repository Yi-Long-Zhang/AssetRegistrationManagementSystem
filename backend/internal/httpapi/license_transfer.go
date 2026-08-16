package httpapi

import (
	"encoding/csv"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"

	"github.com/gin-gonic/gin"
)

// DownloadLicenseImportTemplate 下载许可证导入模板（CSV 或 XLSX）。
func (h *Handler) DownloadLicenseImportTemplate(c *gin.Context) {
	if strings.EqualFold(c.Query("format"), "csv") {
		writeLicenseCSV(c, "license-import-template.csv", service.LicenseImportTemplateHeaders(), nil)
		return
	}
	writeAssetXLSX(c, "license-import-template.xlsx", licenseImportTemplateSheets())
}

// ExportLicenses 导出许可证 CSV（不含密钥明文）。
func (h *Handler) ExportLicenses(c *gin.Context) {
	list, err := h.softwareLicenseService().List()
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "导出许可证失败")
		return
	}
	writeLicenseCSV(c, "licenses-export.csv", service.LicenseExportHeaders(), licenseExportRows(list))
}

// ImportLicenses 批量导入许可证（CSV/XLSX，密钥加密存储，按软件名+厂商去重更新）。
func (h *Handler) ImportLicenses(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "请选择导入文件")
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
		errorJSON(c, http.StatusBadRequest, "导入文件解析失败: "+err.Error())
		return
	}
	created, updated, err := h.softwareLicenseService().ImportRows(records)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "导入失败: "+err.Error())
		return
	}
	h.audit(currentUser(c).ID, "software_license", 0, "import", fmt.Sprintf("created=%d updated=%d", created, updated))
	c.JSON(http.StatusOK, gin.H{"created": created, "updated": updated})
}

func writeLicenseCSV(c *gin.Context, filename string, headers []string, rows [][]string) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	_, _ = c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(c.Writer)
	_ = writer.Write(headers)
	for _, row := range rows {
		_ = writer.Write(row)
	}
	writer.Flush()
}

func licenseExportRows(list []model.SoftwareLicense) [][]string {
	rows := make([][]string, 0, len(list))
	for _, l := range list {
		asset := ""
		if l.Asset != nil {
			asset = l.Asset.Hostname + " / " + l.Asset.IP
		}
		rows = append(rows, []string{
			l.Name,
			l.Vendor,
			l.Type,
			strconv.Itoa(l.TotalSeats),
			strconv.Itoa(l.UsedSeats),
			formatDate(l.ExpireDate),
			formatDate(l.PurchaseDate),
			asset,
			l.Remark,
		})
	}
	return rows
}

func licenseImportTemplateSheets() []xlsxSheet {
	blank := make([]string, len(service.LicenseImportTemplateHeaders()))
	return []xlsxSheet{
		{
			Name: "软件许可导入模板",
			Rows: [][]string{
				service.LicenseImportTemplateHeaders(),
				blank,
			},
		},
		{
			Name: "填写说明",
			Rows: [][]string{
				{"项目", "说明"},
				{"必填字段", "软件名；新增行的许可证密钥必填"},
				{"导入规则", "按「软件名+厂商」匹配：已存在则更新元数据（密钥留空不修改），不存在则新增"},
				{"类型", "commercial / open-source / subscription / other，兼容中文：商业授权/开源/订阅制/其他"},
				{"关联资产", "填写资产的主机名或 IP 地址，未匹配到资产会导入失败"},
				{"导出说明", "导出 CSV 不包含许可证密钥明文，查看请使用列表中的「查看密钥」"},
			},
		},
	}
}
