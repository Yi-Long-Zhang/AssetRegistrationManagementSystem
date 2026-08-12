package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// inspectionRuleRequest 巡检规则请求体
type inspectionRuleRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Frequency   string `json:"frequency" binding:"required"` // daily/weekly/monthly
	DayOfWeek   int    `json:"dayOfWeek"`
	DayOfMonth  int    `json:"dayOfMonth"`
	TimeOfDay   string `json:"timeOfDay" binding:"required"` // "HH:MM"
	AssigneeID  uint   `json:"assigneeId" binding:"required"`
	Enabled     bool   `json:"enabled"`
}

func validateInspectionRule(req inspectionRuleRequest) string {
	switch req.Frequency {
	case service.InspectionDaily, service.InspectionWeekly, service.InspectionMonthly:
	default:
		return "频率必须是 daily/weekly/monthly"
	}
	if _, _, ok := parseTimeHHMM(req.TimeOfDay); !ok {
		return "执行时间格式应为 HH:MM"
	}
	if req.Frequency == service.InspectionWeekly && (req.DayOfWeek < 0 || req.DayOfWeek > 6) {
		return "周执行日应为 0-6（0=周日）"
	}
	if req.Frequency == service.InspectionMonthly && (req.DayOfMonth < 1 || req.DayOfMonth > 31) {
		return "月执行日应为 1-31"
	}
	if req.AssigneeID == 0 {
		return "巡检执行人不能为空"
	}
	return ""
}

// ListInspectionRules 查询巡检规则列表。
// @Summary 巡检规则列表
// @Description 查询定期巡检规则（admin）
// @Tags inspection
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /inspection/rules [get]
func (h *Handler) ListInspectionRules(c *gin.Context) {
	var rules []model.InspectionRule
	if err := h.db.Preload("Assignee").Order("id desc").Find(&rules).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询巡检规则失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rules})
}

// CreateInspectionRule 新增巡检规则。
// @Summary 新增巡检规则
// @Description 创建定期巡检规则（admin）
// @Tags inspection
// @Accept json
// @Produce json
// @Param body body inspectionRuleRequest true "巡检规则"
// @Success 201 {object} model.InspectionRule
// @Security BearerAuth
// @Router /inspection/rules [post]
func (h *Handler) CreateInspectionRule(c *gin.Context) {
	var req inspectionRuleRequest
	if !bind(c, &req) {
		return
	}
	if msg := validateInspectionRule(req); msg != "" {
		errorJSON(c, http.StatusBadRequest, msg)
		return
	}
	if !h.validAssignee(req.AssigneeID) {
		errorJSON(c, http.StatusBadRequest, "巡检执行人必须是有效启用用户")
		return
	}
	rule := model.InspectionRule{
		Name:        strings.TrimSpace(req.Name),
		Description: req.Description,
		Frequency:   req.Frequency,
		DayOfWeek:   req.DayOfWeek,
		DayOfMonth:  req.DayOfMonth,
		TimeOfDay:   req.TimeOfDay,
		AssigneeID:  req.AssigneeID,
		Enabled:     req.Enabled,
	}
	if err := h.db.Create(&rule).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "创建巡检规则失败: "+err.Error())
		return
	}
	h.db.Preload("Assignee").First(&rule, rule.ID)
	c.JSON(http.StatusCreated, rule)
}

// UpdateInspectionRule 更新巡检规则。
// @Summary 更新巡检规则
// @Description 更新定期巡检规则（admin）
// @Tags inspection
// @Accept json
// @Produce json
// @Param id path int true "规则 ID"
// @Param body body inspectionRuleRequest true "巡检规则"
// @Success 200 {object} model.InspectionRule
// @Security BearerAuth
// @Router /inspection/rules/{id} [put]
func (h *Handler) UpdateInspectionRule(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req inspectionRuleRequest
	if !bind(c, &req) {
		return
	}
	if msg := validateInspectionRule(req); msg != "" {
		errorJSON(c, http.StatusBadRequest, msg)
		return
	}
	if !h.validAssignee(req.AssigneeID) {
		errorJSON(c, http.StatusBadRequest, "巡检执行人必须是有效启用用户")
		return
	}
	var rule model.InspectionRule
	err := h.db.First(&rule, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		errorJSON(c, http.StatusNotFound, "巡检规则不存在")
		return
	}
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询巡检规则失败")
		return
	}
	rule.Name = strings.TrimSpace(req.Name)
	rule.Description = req.Description
	rule.Frequency = req.Frequency
	rule.DayOfWeek = req.DayOfWeek
	rule.DayOfMonth = req.DayOfMonth
	rule.TimeOfDay = req.TimeOfDay
	rule.AssigneeID = req.AssigneeID
	rule.Enabled = req.Enabled
	if err := h.db.Save(&rule).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "更新巡检规则失败: "+err.Error())
		return
	}
	h.db.Preload("Assignee").First(&rule, rule.ID)
	c.JSON(http.StatusOK, rule)
}

// DeleteInspectionRule 删除巡检规则。
// @Summary 删除巡检规则
// @Description 删除定期巡检规则（admin）
// @Tags inspection
// @Param id path int true "规则 ID"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /inspection/rules/{id} [delete]
func (h *Handler) DeleteInspectionRule(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.db.Delete(&model.InspectionRule{}, id).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "删除巡检规则失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// TestInspectionRule 立即为规则生成一张巡检工单（验证配置）。
// @Summary 试运行巡检规则
// @Description 立即生成一张巡检工单（admin）
// @Tags inspection
// @Param id path int true "规则 ID"
// @Success 200 {object} model.Ticket
// @Security BearerAuth
// @Router /inspection/rules/{id}/test [post]
func (h *Handler) TestInspectionRule(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var rule model.InspectionRule
	if err := h.db.First(&rule, id).Error; err != nil {
		statusForDBError(c, err, "巡检规则不存在")
		return
	}
	ticket, err := service.CreateInspectionTicket(h.db, rule)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "生成巡检工单失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, ticket)
}

// validAssignee 校验执行人是否为有效启用用户。
func (h *Handler) validAssignee(userID uint) bool {
	var count int64
	_ = h.db.Model(&model.User{}).Where("id = ? AND status = ?", userID, "active").Count(&count).Error
	return count > 0
}

// parseTimeHHMM 解析 "HH:MM" 时间格式。
func parseTimeHHMM(value string) (hour, minute int, ok bool) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0, 0, false
	}
	h, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	m, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, false
	}
	return h, m, true
}
