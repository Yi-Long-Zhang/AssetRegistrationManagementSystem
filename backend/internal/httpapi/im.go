package httpapi

import (
	"encoding/json"
	"fmt"
	"log"
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

// GetIMConfig 读取群机器人通知配置（admin）；secret 不回显明文。
// @Summary IM 通知配置
// @Description 读取钉钉/企微/飞书群机器人通知配置（secret 不回显）
// @Tags settings
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /settings/im [get]
// @Security BearerAuth
func (h *Handler) GetIMConfig(c *gin.Context) {
	cfg := h.currentIMConfig()
	c.JSON(http.StatusOK, gin.H{
		"id":        cfg.ID,
		"enabled":   cfg.Enabled,
		"platform":  cfg.Platform,
		"webhook":   cfg.Webhook,
		"hasSecret": cfg.Secret != "",
	})
}

// SaveIMConfig 保存群机器人通知配置（admin）；secret 非空时 AES-GCM 加密存储。
// @Summary 保存 IM 通知配置
// @Description 保存钉钉/企微/飞书群机器人通知配置（启用时必须填写 webhook；secret 加密存储，留空不修改）
// @Tags settings
// @Accept json
// @Produce json
// @Param body body imConfigRequest true "IM 配置"
// @Success 200 {object} map[string]interface{}
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
	if cfg.Enabled && cfg.Webhook == "" {
		errorJSON(c, http.StatusBadRequest, "启用 IM 通知必须填写群机器人 webhook 地址")
		return
	}
	// secret：非空则加密存储（留空表示不修改既有密钥）
	secret := strings.TrimSpace(req.Secret)
	if secret != "" {
		encrypted, err := service.EncryptString(secret, h.cfg.Security.ConfigEncryptionKey)
		if err != nil {
			errorJSON(c, http.StatusBadRequest, "加密 IM 密钥失败: "+err.Error())
			return
		}
		cfg.Secret = encrypted
		cfg.EncryptedSecret = true
	}
	if err := h.db.Save(&cfg).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "保存 IM 配置失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id": cfg.ID, "enabled": cfg.Enabled, "platform": cfg.Platform,
		"webhook": cfg.Webhook, "hasSecret": cfg.Secret != "",
	})
}

// imConfigSecret 解密当前 IM 配置的 secret（未加密或解密失败时返回原值）。
func (h *Handler) imConfigSecret(cfg *model.IMConfig) string {
	if cfg.Secret == "" {
		return ""
	}
	if !cfg.EncryptedSecret {
		return cfg.Secret
	}
	dec, err := service.DecryptString(cfg.Secret, h.cfg.Security.ConfigEncryptionKey)
	if err != nil {
		log.Printf("im config: decrypt secret: %v", err)
		return ""
	}
	return dec
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
	secret := h.imConfigSecret(&cfg)
	text := service.BuildTicketMessage("IM 通知测试",
		"- 平台："+cfg.Platform+"\n- 时间："+nowText()+"\n\n如果收到本消息，说明配置成功。")
	if err := notifier.SendText(cfg.Webhook, secret, "资产管理系统测试", text); err != nil {
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
	// 平台白名单校验
	platform := req.Platform
	switch platform {
	case string(service.IMPlatformDingTalk), string(service.IMPlatformWeCom), string(service.IMPlatformFeishu):
	default:
		errorJSON(c, http.StatusBadRequest, "不支持的 IM 平台")
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
	raw, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	// 飞书 URL 验证：应答 challenge（平台验证阶段无签名）
	var probe struct {
		Challenge string `json:"challenge"`
	}
	if json.Unmarshal(raw, &probe) == nil && probe.Challenge != "" {
		c.JSON(http.StatusOK, gin.H{"challenge": probe.Challenge})
		return
	}
	// 平台原生验签分派（按 URL query / header 特征识别）
	switch {
	case c.Query("msg_signature") != "":
		h.handleWeComCallback(c, raw)
		return
	case c.Query("signature") != "" || c.Query("sign") != "":
		h.handleDingTalkCallback(c, raw)
		return
	case c.GetHeader("X-Lark-Signature") != "":
		h.handleFeishuCallback(c, raw)
		return
	}
	// 兜底：通用共享密钥 HMAC 验签（X-IM-Sign / X-IM-Timestamp）
	h.handleGenericCallback(c, raw)
}

// handleGenericCallback 通用 HMAC 验签路径（原逻辑，供测试与简易接入）。
func (h *Handler) handleGenericCallback(c *gin.Context, raw []byte) {
	cfg := h.currentIMConfig()
	secret := h.imConfigSecret(&cfg)
	if cfg.Enabled && cfg.Webhook != "" && secret != "" {
		if !h.verifyIMSignature(c, secret, raw) {
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid signature"})
			return
		}
	} else {
		c.JSON(http.StatusForbidden, gin.H{"error": "IM 回调验签未配置，拒绝处理"})
		return
	}
	h.dispatchIMAction(c, raw)
}

// handleDingTalkCallback 钉钉自建应用回调：URL 参数 signature/timestamp 用 AppSecret 验签。
func (h *Handler) handleDingTalkCallback(c *gin.Context, raw []byte) {
	cb := h.currentIMCallbackConfig()
	secret := h.imCallbackSecret(cb, "app_secret")
	if !cb.Enabled || secret == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "钉钉回调验签未配置，拒绝处理"})
		return
	}
	timestamp := c.Query("timestamp")
	sign := c.Query("signature")
	if sign == "" {
		sign = c.Query("sign")
	}
	if !service.VerifyDingTalkSignature(secret, timestamp, sign) {
		c.JSON(http.StatusForbidden, gin.H{"error": "钉钉验签失败"})
		return
	}
	h.dispatchIMAction(c, raw)
}

// handleFeishuCallback 飞书事件回调：X-Lark-Signature 验签。
func (h *Handler) handleFeishuCallback(c *gin.Context, raw []byte) {
	cb := h.currentIMCallbackConfig()
	secret := h.imCallbackSecret(cb, "app_secret")
	if !cb.Enabled || secret == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "飞书回调验签未配置，拒绝处理"})
		return
	}
	timestamp := c.GetHeader("X-Lark-Request-Timestamp")
	nonce := c.GetHeader("X-Lark-Request-Nonce")
	signature := c.GetHeader("X-Lark-Signature")
	if !service.VerifyFeishuSignature(secret, timestamp, nonce, raw, signature) {
		c.JSON(http.StatusForbidden, gin.H{"error": "飞书验签失败"})
		return
	}
	h.dispatchIMAction(c, raw)
}

// handleWeComCallback 企微回调：msg_signature 验签 + AES 解密后处理。
func (h *Handler) handleWeComCallback(c *gin.Context, raw []byte) {
	cb := h.currentIMCallbackConfig()
	token := h.imCallbackSecret(cb, "token")
	aesKey := h.imCallbackSecret(cb, "aes_key")
	if !cb.Enabled || token == "" || aesKey == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "企微回调验签未配置，拒绝处理"})
		return
	}
	wc := service.WeComCallback{Token: token, EncodingAESKey: aesKey, CorpID: cb.CorpID}
	msgSignature := c.Query("msg_signature")
	timestamp := c.Query("timestamp")
	nonce := c.Query("nonce")
	// 解密消息
	decrypted, err := h.decryptWeComPayload(wc, raw, msgSignature, timestamp, nonce)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "企微消息解密失败: " + err.Error()})
		return
	}
	h.dispatchIMAction(c, decrypted)
}

// decryptWeComPayload 从企微回调 body 提取 <Encrypt> 密文，验签并解密为明文。
func (h *Handler) decryptWeComPayload(wc service.WeComCallback, raw []byte, msgSignature, timestamp, nonce string) ([]byte, error) {
	encrypt := extractWeComEncrypt(raw)
	if encrypt == "" {
		return nil, fmt.Errorf("未找到加密内容")
	}
	if !wc.VerifyWeComSignature(msgSignature, timestamp, nonce, encrypt) {
		return nil, fmt.Errorf("msg_signature 校验失败")
	}
	return wc.Decrypt(encrypt)
}

// extractWeComEncrypt 从企微回调 body 提取 <Encrypt> 文本；若非 XML 则按 JSON 的 Encrypt 字段。
func extractWeComEncrypt(raw []byte) string {
	s := string(raw)
	if i := strings.Index(s, "<Encrypt>"); i >= 0 {
		j := strings.Index(s, "</Encrypt>")
		if j > i {
			return s[i+len("<Encrypt>") : j]
		}
	}
	var obj struct {
		Encrypt string `json:"Encrypt"`
	}
	if json.Unmarshal(raw, &obj) == nil && obj.Encrypt != "" {
		return obj.Encrypt
	}
	return ""
}

// dispatchIMAction 解析回调 payload 并处理 approve/reject 动作。
func (h *Handler) dispatchIMAction(c *gin.Context, raw []byte) {
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	action, _ := payload["action"].(string)
	if action == "approve" || action == "reject" {
		h.handleIMTicketAction(c, payload, action)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

// verifyIMSignature 委托 service 层纯函数校验签名与时间戳窗口。
func (h *Handler) verifyIMSignature(c *gin.Context, secret string, raw []byte) bool {
	return service.VerifyIMSignature(secret,
		c.GetHeader("X-IM-Sign"), c.GetHeader("X-IM-Timestamp"), raw, 300)
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
