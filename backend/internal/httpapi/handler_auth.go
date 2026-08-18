package httpapi

import (
	"errors"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if !bind(c, &req) {
		return
	}
	username := strings.ToLower(strings.TrimSpace(req.Username))
	clientIP := c.ClientIP()
	if retryAfter, ok := h.limiter.allow(username, clientIP, time.Now()); !ok {
		c.Header("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
		errorJSON(c, http.StatusTooManyRequests, "登录尝试过于频繁，请稍后再试")
		return
	}

	user, ok := h.authenticate(req.Username, req.Password)
	if !ok {
		h.limiter.failure(username, clientIP, time.Now())
		errorJSON(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	h.limiter.success(username)

	token, err := h.issueToken(c, user)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "创建令牌失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "user": user})
}

func (h *Handler) Logout(c *gin.Context) {
	session := currentSession(c)
	if session.ID != "" {
		_ = h.revokeSession(session.ID, currentUser(c).ID, "logout")
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ChangePassword 修改当前用户密码（本地账号），成功后清除强制改密标记。
func (h *Handler) ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"oldPassword" binding:"required"`
		NewPassword string `json:"newPassword" binding:"required"`
	}
	if !bind(c, &req) {
		return
	}
	user := currentUser(c)
	if user.AuthSource == "ad" {
		errorJSON(c, http.StatusBadRequest, "AD 用户密码请在域控修改")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)) != nil {
		errorJSON(c, http.StatusBadRequest, "原密码错误")
		return
	}
	if len(req.NewPassword) < 8 {
		errorJSON(c, http.StatusBadRequest, "新密码长度不能少于 8 位")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "生成密码失败")
		return
	}
	user.PasswordHash = string(hash)
	user.MustChangePassword = false
	user.SessionVersion++
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&user).Error; err != nil {
			return err
		}
		return revokeUserSessions(tx, user.ID, "password_changed", "")
	}); err != nil {
		errorJSON(c, http.StatusBadRequest, "修改密码失败")
		return
	}
	h.audit(user.ID, "user", user.ID, "change_password", "修改密码")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) Me(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"user": currentUser(c)})
}

func (h *Handler) ListRoles(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": h.roles})
}

func (h *Handler) GetADConfig(c *gin.Context) {
	adConfig := h.currentADConfig()
	c.JSON(http.StatusOK, h.adConfigResponse(adConfig))
}

func (h *Handler) SaveADConfig(c *gin.Context) {
	var req adConfigRequest
	if !bind(c, &req) {
		return
	}
	adConfig := h.currentADConfig()
	adConfig.Enabled = req.Enabled
	adConfig.LDAPURL = req.LDAPURL
	adConfig.BaseDN = req.BaseDN
	adConfig.BindDN = req.BindDN
	adConfig.LoginAttribute = defaultString(req.LoginAttribute, "sAMAccountName")
	adConfig.FilterUserObject = req.FilterUserObject
	adConfig.ExcludeDisabled = req.ExcludeDisabled
	adConfig.AdvancedFilter = req.AdvancedFilter
	if req.AdvancedFilter {
		adConfig.UserFilter = defaultString(req.UserFilter, buildADUserFilter(adConfig.LoginAttribute, adConfig.FilterUserObject, adConfig.ExcludeDisabled))
	} else {
		adConfig.UserFilter = buildADUserFilter(adConfig.LoginAttribute, adConfig.FilterUserObject, adConfig.ExcludeDisabled)
	}
	if req.BindPassword != "" {
		encrypted, err := service.EncryptString(req.BindPassword, h.cfg.Security.ConfigEncryptionKey)
		if err != nil {
			errorJSON(c, http.StatusInternalServerError, "加密 Bind 密码失败")
			return
		}
		adConfig.EncryptedBindPassword = encrypted
	}
	if adConfig.ID == 0 && adConfig.EncryptedBindPassword == "" {
		errorJSON(c, http.StatusBadRequest, "首次保存 AD 配置必须填写 Bind 密码")
		return
	}
	if err := h.db.Save(&adConfig).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "保存 AD 配置失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, h.adConfigResponse(adConfig))
}

func (h *Handler) TestADConnection(c *gin.Context) {
	adConfig, bindPassword, ok := h.readyADConfig(c)
	if !ok {
		return
	}
	if err := h.ad.Test(adConfig, bindPassword); err != nil {
		errorJSON(c, http.StatusBadRequest, "AD 连接测试失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) LookupADUser(c *gin.Context) {
	var req adLookupRequest
	if !bind(c, &req) {
		return
	}
	adConfig, bindPassword, ok := h.readyADConfig(c)
	if !ok {
		return
	}
	info, err := h.ad.LookupUser(adConfig, bindPassword, req.Username)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "查询 AD 用户失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, info)
}

func (h *Handler) ImportADUser(c *gin.Context) {
	var req adImportRequest
	if !bind(c, &req) {
		return
	}
	adConfig, bindPassword, ok := h.readyADConfig(c)
	if !ok {
		return
	}
	info, err := h.ad.LookupUser(adConfig, bindPassword, req.Username)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "查询 AD 用户失败: "+err.Error())
		return
	}
	var existing model.User
	if err := h.db.Where("username = ?", info.Username).First(&existing).Error; err == nil && existing.AuthSource != "ad" {
		errorJSON(c, http.StatusBadRequest, "同名本地用户已存在，不能导入为 AD 用户")
		return
	}
	role := req.Role
	if role == "" {
		role = model.RoleApplicant
	}
	status := defaultString(req.Status, "active")
	user := existing
	if user.ID == 0 {
		user = model.User{Username: info.Username, Role: role, Status: status, PasswordHash: "AD_AUTH"}
	}
	service.ApplyADInfo(&user, info)
	user.Role = role
	user.Status = status
	if err := h.db.Save(&user).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "导入 AD 用户失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, user)
}

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

func (h *Handler) ListTicketTypeApprovers(c *gin.Context) {
	var items []model.TicketTypeApprover
	if err := h.db.Preload("Approver").Order("type asc").Find(&items).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询审批配置失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) SetTicketTypeApprover(c *gin.Context) {
	ticketType := model.TicketType(c.Param("type"))
	var req ticketTypeApproverRequest
	if !bind(c, &req) {
		return
	}
	var approver model.User
	if err := h.db.First(&approver, req.ApproverID).Error; err != nil {
		statusForDBError(c, err, "审批人不存在")
		return
	}
	if approver.Role != model.RoleApprover && approver.Role != model.RoleAdmin {
		errorJSON(c, http.StatusBadRequest, "审批人必须是审批人或管理员角色")
		return
	}

	var item model.TicketTypeApprover
	err := h.db.Where("type = ?", ticketType).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		item = model.TicketTypeApprover{Type: ticketType, ApproverID: req.ApproverID}
		err = h.db.Create(&item).Error
	} else if err == nil {
		item.ApproverID = req.ApproverID
		err = h.db.Save(&item).Error
	}
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "保存审批配置失败: "+err.Error())
		return
	}
	h.db.Preload("Approver").First(&item, item.ID)
	c.JSON(http.StatusOK, item)
}
