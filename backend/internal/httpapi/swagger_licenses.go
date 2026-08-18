package httpapi

import (
	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"
)

var (
	_ model.User
	_ service.ADUserInfo
)

// @Summary 软件许可证列表
// @Tags software-licenses
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /licenses [get]
func swaggerListSoftwareLicenses() {}

// @Summary 创建软件许可证
// @Tags software-licenses
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body softwareLicenseRequest true "软件许可证"
// @Success 201 {object} model.SoftwareLicense
// @Failure 400 {object} errorResponse
// @Router /licenses [post]
func swaggerCreateSoftwareLicense() {}

// @Summary 更新软件许可证
// @Tags software-licenses
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "许可证 ID"
// @Param body body softwareLicenseRequest true "软件许可证"
// @Success 200 {object} model.SoftwareLicense
// @Failure 400 {object} errorResponse
// @Router /licenses/{id} [put]
func swaggerUpdateSoftwareLicense() {}

// @Summary 删除软件许可证
// @Tags software-licenses
// @Produce json
// @Security BearerAuth
// @Param id path int true "许可证 ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errorResponse
// @Router /licenses/{id} [delete]
func swaggerDeleteSoftwareLicense() {}

// @Summary 查看许可证密钥明文
// @Tags software-licenses
// @Produce json
// @Security BearerAuth
// @Param id path int true "许可证 ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errorResponse
// @Router /licenses/{id}/reveal [post]
func swaggerRevealSoftwareLicense() {}

// @Summary 下载许可证导入模板
// @Tags software-licenses
// @Produce application/octet-stream
// @Security BearerAuth
// @Param format query string false "模板格式 csv 或 xlsx"
// @Success 200 {file} file
// @Router /licenses/template [get]
func swaggerDownloadLicenseImportTemplate() {}

// @Summary 导入软件许可证
// @Tags software-licenses
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "CSV 或 XLSX 文件"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errorResponse
// @Router /licenses/import [post]
func swaggerImportLicenses() {}

// @Summary 导出软件许可证 CSV
// @Tags software-licenses
// @Produce text/csv
// @Security BearerAuth
// @Success 200 {file} file
// @Router /licenses/export [get]
func swaggerExportLicenses() {}

// @Summary 许可证附件列表
// @Tags software-licenses
// @Produce json
// @Security BearerAuth
// @Param id path int true "许可证 ID"
// @Success 200 {object} map[string]interface{}
// @Router /licenses/{id}/attachments [get]
func swaggerListLicenseAttachments() {}

// @Summary 上传许可证附件
// @Tags software-licenses
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param id path int true "许可证 ID"
// @Param file formData file true "附件"
// @Success 201 {object} model.LicenseAttachment
// @Failure 400 {object} errorResponse
// @Router /licenses/{id}/attachments [post]
func swaggerUploadLicenseAttachment() {}

// @Summary 下载许可证附件
// @Tags software-licenses
// @Produce application/octet-stream
// @Security BearerAuth
// @Param id path int true "许可证 ID"
// @Param attachmentId path int true "附件 ID"
// @Success 200 {file} file
// @Router /licenses/{id}/attachments/{attachmentId}/download [get]
func swaggerDownloadLicenseAttachment() {}

// @Summary 删除许可证附件
// @Tags software-licenses
// @Produce json
// @Security BearerAuth
// @Param id path int true "许可证 ID"
// @Param attachmentId path int true "附件 ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errorResponse
// @Router /licenses/{id}/attachments/{attachmentId} [delete]
func swaggerDeleteLicenseAttachment() {}
