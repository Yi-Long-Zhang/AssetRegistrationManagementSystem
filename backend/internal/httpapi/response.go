package httpapi

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"asset-registration-management-system/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (h *Handler) audit(actorID uint, entity string, entityID uint, action, detail string) {
	_ = h.db.Create(&model.AuditLog{
		ActorID:  actorID,
		Entity:   entity,
		EntityID: entityID,
		Action:   action,
		Detail:   detail,
	}).Error
}

func parseID(c *gin.Context) (uint, bool) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效 ID")
		return 0, false
	}
	return uint(id64), true
}

// parseIDPath 解析指定名称的路径参数为 uint。
func parseIDPath(c *gin.Context, name string) (uint, bool) {
	id64, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效 "+name)
		return 0, false
	}
	return uint(id64), true
}

func bind(c *gin.Context, out interface{}) bool {
	if err := c.ShouldBindJSON(out); err != nil {
		log.Printf("bind request: %v", err)
		errorJSON(c, http.StatusBadRequest, "请求参数无效")
		return false
	}
	return true
}

func statusForDBError(c *gin.Context, err error, notFound string) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		errorJSON(c, http.StatusNotFound, notFound)
		return
	}
	log.Printf("database error: %v", err)
	errorJSON(c, http.StatusInternalServerError, "服务内部错误")
}

func errorJSON(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func defaultPriority(value model.Priority) model.Priority {
	if value == "" {
		return model.PriorityNormal
	}
	return value
}
