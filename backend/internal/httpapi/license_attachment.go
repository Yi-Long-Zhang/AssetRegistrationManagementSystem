package httpapi

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"asset-registration-management-system/backend/internal/model"

	"github.com/gin-gonic/gin"
)

// ListLicenseAttachments 列出许可证附件（授权书/合同扫描件等）。
func (h *Handler) ListLicenseAttachments(c *gin.Context) {
	licenseID, ok := parseID(c)
	if !ok {
		return
	}
	var attachments []model.LicenseAttachment
	if err := h.db.Preload("Uploader").Where("license_id = ?", licenseID).Order("id desc").Find(&attachments).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询附件失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": attachments})
}

// UploadLicenseAttachment 上传许可证附件，存储于附件目录 licenses/<licenseId>/ 下。
func (h *Handler) UploadLicenseAttachment(c *gin.Context) {
	licenseID, ok := parseID(c)
	if !ok {
		return
	}
	var lic model.SoftwareLicense
	if err := h.db.First(&lic, licenseID).Error; err != nil {
		statusForDBError(c, err, "许可证不存在")
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "请选择上传文件")
		return
	}
	const maxUploadSize = 20 << 20 // 20MB
	if file.Size > maxUploadSize {
		errorJSON(c, http.StatusBadRequest, "附件大小不能超过 20MB")
		return
	}
	storedName := uniqueStoredName(file.Filename)
	// 独立子目录，避免与工单附件（attachments/<ticketId>/）数字目录冲突
	dir := filepath.Join(h.cfg.Storage.AttachmentDir, "licenses", fmt.Sprint(licenseID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		errorJSON(c, http.StatusInternalServerError, "创建附件目录失败")
		return
	}
	storagePath := filepath.Join(dir, storedName)
	if err := c.SaveUploadedFile(file, storagePath); err != nil {
		errorJSON(c, http.StatusInternalServerError, "保存附件失败")
		return
	}
	user := currentUser(c)
	attachment := model.LicenseAttachment{
		LicenseID:    licenseID,
		UploaderID:   user.ID,
		OriginalName: filepath.Base(file.Filename),
		StoredName:   storedName,
		StoragePath:  storagePath,
		Size:         file.Size,
		ContentType:  file.Header.Get("Content-Type"),
	}
	if err := h.db.Create(&attachment).Error; err != nil {
		_ = os.Remove(storagePath)
		errorJSON(c, http.StatusBadRequest, "保存附件元数据失败: "+err.Error())
		return
	}
	h.audit(user.ID, "software_license", licenseID, "attach", attachment.OriginalName)
	h.db.Preload("Uploader").First(&attachment, attachment.ID)
	c.JSON(http.StatusCreated, attachment)
}

// DownloadLicenseAttachment 下载许可证附件。
func (h *Handler) DownloadLicenseAttachment(c *gin.Context) {
	licenseID, ok := parseID(c)
	if !ok {
		return
	}
	attachmentID, err := strconv.ParseUint(c.Param("attachmentId"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效附件 ID")
		return
	}
	var attachment model.LicenseAttachment
	if err := h.db.Where("license_id = ? AND id = ?", licenseID, attachmentID).First(&attachment).Error; err != nil {
		statusForDBError(c, err, "附件不存在")
		return
	}
	c.FileAttachment(attachment.StoragePath, attachment.OriginalName)
}

// DeleteLicenseAttachment 删除许可证附件（删除数据库记录并清理文件）。
func (h *Handler) DeleteLicenseAttachment(c *gin.Context) {
	licenseID, ok := parseID(c)
	if !ok {
		return
	}
	attachmentID, err := strconv.ParseUint(c.Param("attachmentId"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效附件 ID")
		return
	}
	var attachment model.LicenseAttachment
	if err := h.db.Where("license_id = ? AND id = ?", licenseID, attachmentID).First(&attachment).Error; err != nil {
		statusForDBError(c, err, "附件不存在")
		return
	}
	if err := h.db.Delete(&attachment).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "删除附件失败")
		return
	}
	_ = os.Remove(attachment.StoragePath)
	h.audit(currentUser(c).ID, "software_license", licenseID, "delete_attach", attachment.OriginalName)
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}
