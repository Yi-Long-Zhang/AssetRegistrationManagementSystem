package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"

	"github.com/gin-gonic/gin"
)

// imConfigRequest IM 群机器人配置请求
type imConfigRequest struct {
	Enabled  bool   `json:"enabled"`
	Platform string `json:"platform"`
	Webhook  string `json:"webhook"`
	Secret   string `json:"secret"`
}

// GetIMConfig 读取群机器人通知配置（admin）。
// @Summary IM 通知配置
// @Description 读取钉钉/企微/飞书群机器人通知配置
// @Tags settings
// @Produce json
// @Success 200 {object} model.IMConfig
// @Router /settings/im [get]
// @Security BearerAuth
func (h *Handler) GetIMConfig(c *gin.Context) {
	cfg := h.currentIMConfig()
	c.JSON(http.StatusOK, cfg)
}

// SaveIMConfig 保存群机器人通知配置（admin）。
// @Summary 保存 IM 通知配置
// @Description 保存钉钉/企微/飞书群机器人通知配置（启用时必须填写 webhook）
// @Tags settings
// @Accept json
// @Produce json
// @Param body body imConfigRequest true "IM 配置"
// @Success 200 {object} model.IMConfig
// @Failure 400 {object} map[string]string
// @Router /settings/im [put]
// @Security BearerAuth
func (h *Handler) SaveIMConfig(c *gin.Context) {
	var req imConfigRequest
	if !bind(c, &req) {
		return
	}
	cfg := h.currentIMConfig()
	cfg.Enabled = req.Enabled
	cfg.Platform = defaultString(strings.TrimSpace(req.Platform), string(service.IMPlatformDingTalk))
	cfg.Webhook = strings.TrimSpace(req.Webhook)
	cfg.Secret = strings.TrimSpace(req.Secret)
	if cfg.Enabled && cfg.Webhook == "" {
		errorJSON(c, http.StatusBadRequest, "启用 IM 通知必须填写群机器人 webhook 地址")
		return
	}
	if err := h.db.Save(&cfg).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "保存 IM 配置失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// TestIMConfig 发送测试卡片到群机器人（admin）。
// @Summary 测试 IM 通知
// @Description 使用当前配置发送一条测试消息到群机器人
// @Tags settings
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Router /settings/im/test [post]
// @Security BearerAuth
func (h *Handler) TestIMConfig(c *gin.Context) {
	cfg := h.currentIMConfig()
	if !cfg.Enabled || cfg.Webhook == "" {
		errorJSON(c, http.StatusBadRequest, "请先启用并填写群机器人 webhook")
		return
	}
	notifier := h.im
	if notifier == nil {
		notifier = service.NewIMNotifier()
	}
	text := service.BuildTicketMessage("IM 通知测试",
		"- 平台："+cfg.Platform+"\n- 时间："+nowText()+"\n\n如果收到本消息，说明配置成功。")
	if err := notifier.SendText(cfg.Webhook, cfg.Secret, "资产管理系统测试", text); err != nil {
		errorJSON(c, http.StatusBadRequest, "发送测试失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"sent": true})
}

// currentIMConfig 读取 IM 配置（不存在时返回默认关闭配置）。
func (h *Handler) currentIMConfig() model.IMConfig {
	var cfg model.IMConfig
	if err := h.db.First(&cfg).Error; err != nil {
		return model.IMConfig{Platform: string(service.IMPlatformDingTalk)}
	}
	return cfg
}

func nowText() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// imBindingRequest 绑定请求
type imBindingRequest struct {
	UserID   uint   `json:"userId" binding:"required"`
	Platform string `json:"platform" binding:"required"`
	IMUserID string `json:"imUserId" binding:"required"`
}

// ListIMBindings 查询 IM 用户绑定列表（admin）。
// @Summary IM 用户绑定列表
// @Description 查询 IM 用户与系统用户映射（用于 IM 回调鉴权）
// @Tags settings
// @Produce json
// @Success 200 {array} model.IMBinding
// @Router /settings/im/bindings [get]
// @Security BearerAuth
func (h *Handler) ListIMBindings(c *gin.Context) {
	var bindings []model.IMBinding
	if err := h.db.Preload("User").Order("id DESC").Find(&bindings).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询 IM 绑定失败")
		return
	}
	c.JSON(http.StatusOK, bindings)
}

// SaveIMBinding 新增/更新 IM 用户绑定（admin）。
// @Summary 新增/更新 IM 绑定
// @Description 建立 IM 用户与系统用户映射（同一用户仅保留一个平台绑定）
// @Tags settings
// @Accept json
// @Produce json
// @Param body body imBindingRequest true "绑定信息"
// @Success 200 {object} model.IMBinding
// @Failure 400 {object} map[string]string
// @Router /settings/im/bindings [put]
// @Security BearerAuth
func (h *Handler) SaveIMBinding(c *gin.Context) {
	var req imBindingRequest
	if !bind(c, &req) {
		return
	}
	var user model.User
	if err := h.db.First(&user, req.UserID).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "系统用户不存在")
		return
	}
	var binding model.IMBinding
	if err := h.db.Where("user_id = ?", req.UserID).First(&binding).Error; err != nil {
		binding = model.IMBinding{UserID: req.UserID}
	}
	binding.Platform = req.Platform
	binding.IMUserID = strings.TrimSpace(req.IMUserID)
	if binding.IMUserID == "" {
		errorJSON(c, http.StatusBadRequest, "IM 用户标识不能为空")
		return
	}
	if err := h.db.Save(&binding).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "保存绑定失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, binding)
}

// DeleteIMBinding 删除 IM 用户绑定（admin）。
// @Summary 删除 IM 绑定
// @Description 删除指定用户的 IM 绑定
// @Tags settings
// @Param userId path int true "系统用户 ID"
// @Success 204
// @Router /settings/im/bindings/{userId} [delete]
// @Security BearerAuth
func (h *Handler) DeleteIMBinding(c *gin.Context) {
	userID, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.db.Where("user_id = ?", userID).Delete(&model.IMBinding{}).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "删除绑定失败: "+err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

// IMCallback 接收 IM 平台回调（群机器人阶段仅处理平台 URL 验证与健康检查；
// 交互按钮审批需平台自建应用 + 公网回调，验签逻辑在接入时补充）。
// @Summary IM 平台回调
// @Description 接收 IM 平台事件回调（飞书 challenge 验证；approve/reject 交互动作经用户绑定鉴权后流转工单）
// @Tags system
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /im/callback [post]
func (h *Handler) IMCallback(c *gin.Context) {
	var payload map[string]interface{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "invalid payload"})
		return
	}
	// 飞书 URL 验证：应答 challenge
	if ch, ok := payload["challenge"].(string); ok && ch != "" {
		c.JSON(http.StatusOK, gin.H{"challenge": ch})
		return
	}
	// 交互动作：approve/reject（自建应用回调；经 IM 用户绑定鉴权后流转工单）
	action, _ := payload["action"].(string)
	if action == "approve" || action == "reject" {
		h.handleIMTicketAction(c, payload, action)
		return
	}
	// 其它事件暂记录，等待自建应用接入
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

// handleIMTicketAction 处理 IM 回调的工单审批动作：查绑定 → 鉴权 → 状态流转 → 记录。
func (h *Handler) handleIMTicketAction(c *gin.Context, payload map[string]interface{}, action string) {
	ticketID, _ := payload["ticketId"].(float64)
	imUserID, _ := payload["imUserId"].(string)
	platform, _ := payload["platform"].(string)
	remark, _ := payload["remark"].(string)
	if ticketID == 0 || imUserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ticketId 与 imUserId 必填"})
		return
	}
	var binding model.IMBinding
	if err := h.db.Preload("User").Where("im_user_id = ? AND platform = ?", imUserID, platform).First(&binding).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "IM 用户未绑定系统账号"})
		return
	}
	var ticket model.Ticket
	if err := h.db.First(&ticket, uint(ticketID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "工单不存在"})
		return
	}
	if ticket.Status != model.TicketStatusPendingApproval {
		c.JSON(http.StatusBadRequest, gin.H{"error": "工单当前状态不可审批"})
		return
	}
	user := binding.User
	from := ticket.Status
	if action == "approve" {
		if !h.approveCurrentStep(c, &ticket, user, remark) {
			return // 错误已通过 errorJSON 返回
		}
		ticket.Status = model.TicketStatusApproved
	} else {
		if !h.rejectCurrentStep(c, &ticket, user, remark) {
			return
		}
		ticket.Status = model.TicketStatusRejected
	}
	if err := h.db.Save(&ticket).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "更新工单失败: "+err.Error())
		return
	}
	h.addRecord(ticket.ID, user.ID, action, from, ticket.Status, remark)
	h.notifyTicketIM(&ticket, "IM 审批"+action,
		fmt.Sprintf("- 工单：#%d %s\n- 处理人：%s", ticket.ID, ticket.Title, user.Username))
	c.JSON(http.StatusOK, gin.H{"code": 0, "ticketId": ticket.ID, "status": ticket.Status})
}
