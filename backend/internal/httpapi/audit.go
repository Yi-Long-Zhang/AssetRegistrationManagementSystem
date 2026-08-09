package httpapi

import (
	"net/http"
	"strings"

	"asset-registration-management-system/backend/internal/model"

	"github.com/gin-gonic/gin"
)

// ListAuditLogs 分页查询操作审计日志（admin），支持按实体/动作过滤。
// @Summary 操作审计日志
// @Description 分页查询操作审计日志（admin），支持 entity/action 过滤
// @Tags system
// @Produce json
// @Param page query int false "页码（默认 1）"
// @Param pageSize query int false "每页条数（默认 20，最大 200）"
// @Param entity query string false "实体（asset/ticket/user/...）"
// @Param action query string false "动作（create/update/delete/alert/...）"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /audit-logs [get]
// @Security BearerAuth
func (h *Handler) ListAuditLogs(c *gin.Context) {
	page, pageSize := assetPagination(c)
	db := h.db.Model(&model.AuditLog{})
	if entity := strings.TrimSpace(c.Query("entity")); entity != "" {
		db = db.Where("entity = ?", entity)
	}
	if action := strings.TrimSpace(c.Query("action")); action != "" {
		db = db.Where("action = ?", action)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询审计日志失败")
		return
	}
	var logs []model.AuditLog
	if err := db.Preload("Actor").Order("id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询审计日志失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": logs, "total": total, "page": page, "pageSize": pageSize})
}
