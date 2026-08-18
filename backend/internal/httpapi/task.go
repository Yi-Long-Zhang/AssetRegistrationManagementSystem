package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (h *Handler) ListBackgroundTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	items, total, err := h.tasks.List(c.Query("kind"), c.Query("status"), page, pageSize)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询后台任务失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "pageSize": pageSize})
}

func (h *Handler) GetBackgroundTask(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	task, err := h.tasks.Get(id)
	if err != nil {
		statusForDBError(c, err, "后台任务不存在")
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *Handler) RetryBackgroundTask(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	task, err := h.tasks.Retry(c.Request.Context(), id)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "任务重试失败: "+err.Error())
		return
	}
	h.audit(currentUser(c).ID, "background_task", id, "retry", task.Kind)
	c.JSON(http.StatusOK, task)
}

func (h *Handler) AcknowledgeBackgroundTask(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.tasks.Acknowledge(id, currentUser(c).ID); err != nil {
		errorJSON(c, http.StatusBadRequest, "确认任务失败")
		return
	}
	h.audit(currentUser(c).ID, "background_task", id, "acknowledge", "")
	c.JSON(http.StatusOK, gin.H{"acknowledged": true})
}
