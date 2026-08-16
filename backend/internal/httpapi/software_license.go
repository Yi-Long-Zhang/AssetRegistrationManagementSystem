package httpapi

import (
	"net/http"
	"strings"
	"time"

	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"

	"github.com/gin-gonic/gin"
)

// softwareLicenseRequest 软件许可证请求体（licenseKey 为明文，仅创建/更新时提交）。
type softwareLicenseRequest struct {
	AssetID      *uint  `json:"assetId"`
	Name         string `json:"name" binding:"required"`
	Vendor       string `json:"vendor"`
	Type         string `json:"type"`
	LicenseKey   string `json:"licenseKey"`
	TotalSeats   int    `json:"totalSeats"`
	UsedSeats    int    `json:"usedSeats"`
	ExpireDate   string `json:"expireDate"`
	PurchaseDate string `json:"purchaseDate"`
	Remark       string `json:"remark"`
}

func (h *Handler) softwareLicenseService() *service.SoftwareLicenseService {
	return service.NewSoftwareLicenseService(h.db, h.cfg.Security.ConfigEncryptionKey)
}

func normalizeLicenseType(t string) string {
	return model.NormalizeLicenseType(t)
}

// parseOptionalDate 解析 YYYY-MM-DD 日期字符串，空串或非法值返回 nil。
func parseOptionalDate(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	t, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return nil
	}
	return &t
}

// ListSoftwareLicenses 列出全部软件许可证（不含密钥明文）。
func (h *Handler) ListSoftwareLicenses(c *gin.Context) {
	list, err := h.softwareLicenseService().List()
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询软件许可证失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list})
}

// CreateSoftwareLicense 创建软件许可证（加密存储许可证密钥）。
func (h *Handler) CreateSoftwareLicense(c *gin.Context) {
	var req softwareLicenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数无效")
		return
	}
	if req.LicenseKey == "" {
		errorJSON(c, http.StatusBadRequest, "licenseKey 不能为空")
		return
	}
	lic := &model.SoftwareLicense{
		AssetID:      req.AssetID,
		Name:         req.Name,
		Vendor:       req.Vendor,
		Type:         normalizeLicenseType(req.Type),
		TotalSeats:   req.TotalSeats,
		UsedSeats:    req.UsedSeats,
		ExpireDate:   parseOptionalDate(req.ExpireDate),
		PurchaseDate: parseOptionalDate(req.PurchaseDate),
		Remark:       req.Remark,
	}
	if err := h.softwareLicenseService().Create(lic, req.LicenseKey); err != nil {
		errorJSON(c, http.StatusInternalServerError, "创建软件许可证失败")
		return
	}
	h.audit(currentUser(c).ID, "software_license", lic.ID, "create", lic.Name)
	c.JSON(http.StatusCreated, lic)
}

// UpdateSoftwareLicense 更新软件许可证（licenseKey 非空时重新加密）。
func (h *Handler) UpdateSoftwareLicense(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req softwareLicenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数无效")
		return
	}
	lic := &model.SoftwareLicense{
		AssetID:      req.AssetID,
		Name:         req.Name,
		Vendor:       req.Vendor,
		Type:         normalizeLicenseType(req.Type),
		TotalSeats:   req.TotalSeats,
		UsedSeats:    req.UsedSeats,
		ExpireDate:   parseOptionalDate(req.ExpireDate),
		PurchaseDate: parseOptionalDate(req.PurchaseDate),
		Remark:       req.Remark,
	}
	if err := h.softwareLicenseService().Update(id, lic, req.LicenseKey); err != nil {
		errorJSON(c, http.StatusInternalServerError, "更新软件许可证失败")
		return
	}
	h.audit(currentUser(c).ID, "software_license", id, "update", lic.Name)
	c.JSON(http.StatusOK, lic)
}

// DeleteSoftwareLicense 删除软件许可证。
func (h *Handler) DeleteSoftwareLicense(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.softwareLicenseService().Delete(id); err != nil {
		errorJSON(c, http.StatusInternalServerError, "删除软件许可证失败")
		return
	}
	h.audit(currentUser(c).ID, "software_license", id, "delete", "")
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// RevealSoftwareLicense 解密返回明文许可证密钥（写审计）。
func (h *Handler) RevealSoftwareLicense(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	lic, key, err := h.softwareLicenseService().Reveal(id)
	if err != nil {
		errorJSON(c, http.StatusNotFound, "许可证不存在或解密失败")
		return
	}
	h.audit(currentUser(c).ID, "software_license", id, "reveal", lic.Name)
	c.JSON(http.StatusOK, gin.H{"license": lic, "licenseKey": key})
}
