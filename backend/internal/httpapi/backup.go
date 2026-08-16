package httpapi

import (
	"log"
	"net/http"
	"path/filepath"

	"asset-registration-management-system/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func (h *Handler) backupService() *service.BackupService {
	return service.NewBackupService(h.db, h.cfg.Storage.DatabasePath, h.cfg.Storage.BackupDir, h.cfg.Storage.BackupKeepDays)
}

// ListBackups 列出全部备份（按时间倒序）。
// @Summary 备份列表
// @Description 查询全部数据库备份文件（admin）
// @Tags backups
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /backups [get]
func (h *Handler) ListBackups(c *gin.Context) {
	list, err := h.backupService().List()
	if err != nil {
		log.Printf("list backups: %v", err)
		errorJSON(c, http.StatusInternalServerError, "查询备份失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list})
}

// CreateBackup 手动创建一份备份。
// @Summary 创建备份
// @Description 生成一份 SQLite 一致性快照备份（admin）
// @Tags backups
// @Produce json
// @Success 201 {object} service.BackupInfo
// @Failure 500 {object} errorResponse
// @Security BearerAuth
// @Router /backups [post]
func (h *Handler) CreateBackup(c *gin.Context) {
	info, err := h.backupService().Create()
	if err != nil {
		log.Printf("create backup: %v", err)
		errorJSON(c, http.StatusInternalServerError, "备份失败")
		return
	}
	c.JSON(http.StatusCreated, info)
}

// DeleteBackup 删除指定备份。
// @Summary 删除备份
// @Description 删除指定备份文件（admin）
// @Tags backups
// @Produce json
// @Param name path string true "备份文件名"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errorResponse
// @Security BearerAuth
// @Router /backups/{name} [delete]
func (h *Handler) DeleteBackup(c *gin.Context) {
	name := c.Param("name")
	if err := h.backupService().Delete(name); err != nil {
		errorJSON(c, http.StatusBadRequest, "删除备份失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// DownloadBackup 下载指定备份文件。
// @Summary 下载备份
// @Description 下载指定备份数据库文件（admin）
// @Tags backups
// @Produce application/octet-stream
// @Param name path string true "备份文件名"
// @Success 200 {file} file
// @Failure 400 {object} errorResponse
// @Security BearerAuth
// @Router /backups/{name}/download [get]
func (h *Handler) DownloadBackup(c *gin.Context) {
	name := c.Param("name")
	if filepath.Base(name) != name {
		errorJSON(c, http.StatusBadRequest, "非法备份名")
		return
	}
	c.FileAttachment(filepath.Join(h.cfg.Storage.BackupDir, name), name)
}

// RestoreBackup 校验并标记待恢复（重启后端后生效）。
// @Summary 恢复备份
// @Description 校验备份并标记待恢复，重启后端后生效（admin）
// @Tags backups
// @Produce json
// @Param name path string true "备份文件名"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errorResponse
// @Security BearerAuth
// @Router /backups/{name}/restore [post]
func (h *Handler) RestoreBackup(c *gin.Context) {
	name := c.Param("name")
	if err := h.backupService().Restore(name); err != nil {
		errorJSON(c, http.StatusBadRequest, "恢复失败")
		return
	}
	h.audit(currentUser(c).ID, "backup", 0, "restore", name)
	c.JSON(http.StatusOK, gin.H{"restored": true, "message": "已标记恢复，重启后端后生效"})
}
