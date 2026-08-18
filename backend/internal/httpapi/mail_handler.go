package httpapi

import (
	"net/http"
	"net/mail"
	"strings"

	"asset-registration-management-system/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetMailConfig(c *gin.Context) {
	mailConfig := h.currentMailConfig()
	c.JSON(http.StatusOK, h.mailConfigResponse(mailConfig))
}

func (h *Handler) SaveMailConfig(c *gin.Context) {
	var req mailConfigRequest
	if !bind(c, &req) {
		return
	}
	mailConfig := h.currentMailConfig()
	mailConfig.Enabled = req.Enabled
	mailConfig.SMTPHost = strings.TrimSpace(req.SMTPHost)
	mailConfig.SMTPPort = req.SMTPPort
	if mailConfig.SMTPPort == 0 {
		mailConfig.SMTPPort = 25
	}
	mailConfig.Username = strings.TrimSpace(req.Username)
	mailConfig.FromAddress = strings.TrimSpace(req.FromAddress)
	mailConfig.FromName = strings.TrimSpace(defaultString(req.FromName, "资产管理系统"))
	mailConfig.UseTLS = req.UseTLS
	mailConfig.StartTLS = req.StartTLS
	if mailConfig.Enabled {
		if mailConfig.SMTPHost == "" || mailConfig.FromAddress == "" {
			errorJSON(c, http.StatusBadRequest, "启用邮件通知必须填写 SMTP 地址和发件邮箱")
			return
		}
		if _, err := mail.ParseAddress(mailConfig.FromAddress); err != nil {
			errorJSON(c, http.StatusBadRequest, "发件邮箱格式无效")
			return
		}
	}
	if req.Password != "" {
		encrypted, err := service.EncryptString(req.Password, h.cfg.Security.ConfigEncryptionKey)
		if err != nil {
			errorJSON(c, http.StatusInternalServerError, "加密 SMTP 密码失败")
			return
		}
		mailConfig.EncryptedPassword = encrypted
	}
	if err := h.db.Save(&mailConfig).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "保存邮件配置失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, h.mailConfigResponse(mailConfig))
}

func (h *Handler) TestMailConfig(c *gin.Context) {
	var req mailTestRequest
	_ = c.ShouldBindJSON(&req)
	mailConfig, password, ok := h.readyMailConfig(c)
	if !ok {
		return
	}
	recipient := strings.TrimSpace(req.Recipient)
	if recipient == "" {
		recipient = mailConfig.FromAddress
	}
	address, err := mail.ParseAddress(recipient)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "测试收件邮箱格式无效")
		return
	}
	if err := h.mail.Send(mailConfig, password, service.MailMessage{
		To:      []mail.Address{*address},
		Subject: "资产管理系统邮件测试",
		Body:    "这是一封来自资产管理系统的邮件测试。收到此邮件表示 SMTP 配置可用。",
	}); err != nil {
		errorJSON(c, http.StatusBadRequest, "邮件发送测试失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
