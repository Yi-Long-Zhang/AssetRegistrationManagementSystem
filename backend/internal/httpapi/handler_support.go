package httpapi

import (
	"errors"
	"fmt"
	"log"
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

func (h *Handler) authenticate(username, password string) (model.User, bool) {
	var user model.User
	err := h.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return model.User{}, false
	}
	if user.Status != "active" {
		return model.User{}, false
	}
	// 登录锁定检查（暴力破解防护）
	if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
		return model.User{}, false
	}
	authMode := strings.ToLower(defaultString(h.cfg.Auth.Mode, "mixed"))
	if user.AuthSource == "" {
		user.AuthSource = "local"
	}
	if user.AuthSource == "local" {
		if authMode == "ldap" {
			return model.User{}, false
		}
		if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
			h.recordLoginFailure(&user)
			return model.User{}, false
		}
		h.clearLoginFailure(&user)
		now := time.Now()
		user.LastLoginAt = &now
		_ = h.db.Save(&user).Error
		return user, true
	}
	if user.AuthSource == "ad" {
		if authMode == "local" {
			return model.User{}, false
		}
		adConfig, bindPassword, ok := h.adConfigForAuth()
		if !ok {
			return model.User{}, false
		}
		info, err := h.ad.Authenticate(adConfig, bindPassword, username, password)
		if err != nil {
			h.recordLoginFailure(&user)
			return model.User{}, false
		}
		h.clearLoginFailure(&user)
		service.ApplyADInfo(&user, info)
		if err := h.db.Save(&user).Error; err != nil {
			return model.User{}, false
		}
		return user, true
	}
	return model.User{}, false
}

// recordLoginFailure 记录一次登录失败，连续 5 次失败锁定 15 分钟。
func (h *Handler) recordLoginFailure(user *model.User) {
	user.FailedAttempts++
	if user.FailedAttempts >= 5 {
		lockUntil := time.Now().Add(15 * time.Minute)
		user.LockedUntil = &lockUntil
		user.FailedAttempts = 0
	}
	_ = h.db.Model(user).Updates(map[string]any{
		"failed_attempts": user.FailedAttempts,
		"locked_until":    user.LockedUntil,
	}).Error
}

// clearLoginFailure 登录成功后清零失败计数与锁定。
func (h *Handler) clearLoginFailure(user *model.User) {
	if user.FailedAttempts != 0 || user.LockedUntil != nil {
		user.FailedAttempts = 0
		user.LockedUntil = nil
		_ = h.db.Model(user).Updates(map[string]any{
			"failed_attempts": 0,
			"locked_until":    nil,
		}).Error
	}
}

func (h *Handler) adConfigForAuth() (model.ADConfig, string, bool) {
	adConfig := h.currentADConfig()
	if adConfig.ID == 0 || !adConfig.Enabled {
		return model.ADConfig{}, "", false
	}
	bindPassword, err := service.DecryptString(adConfig.EncryptedBindPassword, h.cfg.Security.ConfigEncryptionKey)
	if err != nil {
		return model.ADConfig{}, "", false
	}
	return adConfig, bindPassword, true
}

func (h *Handler) currentADConfig() model.ADConfig {
	var adConfig model.ADConfig
	if err := h.db.First(&adConfig).Error; err != nil {
		return defaultADConfig()
	}
	if adConfig.LoginAttribute == "" {
		adConfig.LoginAttribute = "sAMAccountName"
	}
	if adConfig.UserFilter == "" {
		adConfig.UserFilter = buildADUserFilter(adConfig.LoginAttribute, adConfig.FilterUserObject, adConfig.ExcludeDisabled)
	}
	return adConfig
}

func (h *Handler) readyADConfig(c *gin.Context) (model.ADConfig, string, bool) {
	adConfig := h.currentADConfig()
	if adConfig.ID == 0 || !adConfig.Enabled {
		errorJSON(c, http.StatusBadRequest, "AD 配置未启用")
		return model.ADConfig{}, "", false
	}
	bindPassword, err := service.DecryptString(adConfig.EncryptedBindPassword, h.cfg.Security.ConfigEncryptionKey)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "AD Bind 密码解密失败")
		return model.ADConfig{}, "", false
	}
	return adConfig, bindPassword, true
}

func (h *Handler) adConfigResponse(adConfig model.ADConfig) gin.H {
	return gin.H{
		"id":               adConfig.ID,
		"enabled":          adConfig.Enabled,
		"ldapUrl":          adConfig.LDAPURL,
		"baseDn":           adConfig.BaseDN,
		"bindDn":           adConfig.BindDN,
		"loginAttribute":   defaultString(adConfig.LoginAttribute, "sAMAccountName"),
		"filterUserObject": adConfig.FilterUserObject,
		"excludeDisabled":  adConfig.ExcludeDisabled,
		"advancedFilter":   adConfig.AdvancedFilter,
		"userFilter":       defaultString(adConfig.UserFilter, buildADUserFilter(adConfig.LoginAttribute, adConfig.FilterUserObject, adConfig.ExcludeDisabled)),
		"hasBindPassword":  adConfig.EncryptedBindPassword != "",
		"createdAt":        adConfig.CreatedAt,
		"updatedAt":        adConfig.UpdatedAt,
	}
}

func (h *Handler) currentMailConfig() model.MailConfig {
	var mailConfig model.MailConfig
	if err := h.db.First(&mailConfig).Error; err != nil {
		return defaultMailConfig()
	}
	if mailConfig.SMTPPort == 0 {
		mailConfig.SMTPPort = 25
	}
	if mailConfig.FromName == "" {
		mailConfig.FromName = "资产管理系统"
	}
	return mailConfig
}

func defaultMailConfig() model.MailConfig {
	return model.MailConfig{
		SMTPPort: 25,
		FromName: "资产管理系统",
		StartTLS: true,
	}
}

func (h *Handler) readyMailConfig(c *gin.Context) (model.MailConfig, string, bool) {
	mailConfig := h.currentMailConfig()
	if mailConfig.ID == 0 || !mailConfig.Enabled {
		errorJSON(c, http.StatusBadRequest, "邮件配置未启用")
		return model.MailConfig{}, "", false
	}
	password, err := h.mailConfigPassword(mailConfig)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "SMTP 密码解密失败")
		return model.MailConfig{}, "", false
	}
	return mailConfig, password, true
}

func (h *Handler) mailConfigPassword(mailConfig model.MailConfig) (string, error) {
	if mailConfig.EncryptedPassword == "" {
		return "", nil
	}
	return service.DecryptString(mailConfig.EncryptedPassword, h.cfg.Security.ConfigEncryptionKey)
}

func (h *Handler) mailConfigResponse(mailConfig model.MailConfig) gin.H {
	return gin.H{
		"id":          mailConfig.ID,
		"enabled":     mailConfig.Enabled,
		"smtpHost":    mailConfig.SMTPHost,
		"smtpPort":    mailConfig.SMTPPort,
		"username":    mailConfig.Username,
		"fromAddress": mailConfig.FromAddress,
		"fromName":    mailConfig.FromName,
		"useTls":      mailConfig.UseTLS,
		"startTls":    mailConfig.StartTLS,
		"hasPassword": mailConfig.EncryptedPassword != "",
		"createdAt":   mailConfig.CreatedAt,
		"updatedAt":   mailConfig.UpdatedAt,
	}
}

func defaultADConfig() model.ADConfig {
	return model.ADConfig{
		LoginAttribute:   "sAMAccountName",
		FilterUserObject: true,
		ExcludeDisabled:  true,
		UserFilter:       buildADUserFilter("sAMAccountName", true, true),
	}
}

func buildADUserFilter(loginAttribute string, filterUserObject, excludeDisabled bool) string {
	if loginAttribute == "" {
		loginAttribute = "sAMAccountName"
	}
	parts := []string{fmt.Sprintf("(%s=%%s)", loginAttribute)}
	if filterUserObject {
		parts = append([]string{"(objectClass=user)"}, parts...)
	}
	if excludeDisabled {
		parts = append(parts, "(!(userAccountControl:1.2.840.113556.1.4.803:=2))")
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "(&" + strings.Join(parts, "") + ")"
}

func (h *Handler) RequireAnyRole(roles ...model.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUser(c)
		for _, role := range roles {
			if user.Role == role {
				c.Next()
				return
			}
		}
		errorJSON(c, http.StatusForbidden, "没有权限执行该操作")
		c.Abort()
	}
}

func currentUser(c *gin.Context) model.User {
	user, _ := c.Get("user")
	typed, _ := user.(model.User)
	return typed
}

func currentSession(c *gin.Context) model.AuthSession {
	session, _ := c.Get("session")
	typed, _ := session.(model.AuthSession)
	return typed
}

func (h *Handler) findByID(c *gin.Context, id uint, out interface{}) bool {
	if err := h.db.First(out, id).Error; err != nil {
		statusForDBError(c, err, "资源不存在")
		return false
	}
	return true
}

func (h *Handler) findTicketForUser(c *gin.Context, id uint, ticket *model.Ticket) bool {
	if err := h.db.First(ticket, id).Error; err != nil {
		statusForDBError(c, err, "工单不存在")
		return false
	}
	return h.canViewTicket(c, *ticket)
}

func (h *Handler) canViewTicket(c *gin.Context, ticket model.Ticket) bool {
	user := currentUser(c)
	if user.Role == model.RoleAdmin || ticket.ApplicantID == user.ID {
		return true
	}
	if ticket.ExecutorID != nil && *ticket.ExecutorID == user.ID {
		return true
	}
	// 审批人：参与过任一审批节点的（含代理）可查看
	if user.Role == model.RoleApprover {
		var count int64
		_ = h.db.Model(&model.TicketWorkflowStepApprover{}).
			Joins("JOIN ticket_workflow_steps ON ticket_workflow_steps.id = ticket_workflow_step_approvers.step_id").
			Joins("JOIN users ON users.id = ticket_workflow_step_approvers.user_id").
			Where("ticket_workflow_steps.ticket_id = ? AND (ticket_workflow_step_approvers.user_id = ? OR users.proxy_user_id = ?)", ticket.ID, user.ID, user.ID).
			Count(&count).Error
		if count > 0 {
			return true
		}
	}
	// 资产管理员：执行阶段及之后可查看
	if user.Role == model.RoleAssetManager {
		return ticket.Status == model.TicketStatusApproved ||
			ticket.Status == model.TicketStatusInProgress ||
			ticket.Status == model.TicketStatusPendingAcceptance ||
			ticket.Status == model.TicketStatusClosed
	}
	errorJSON(c, http.StatusForbidden, "没有权限查看该工单")
	return false
}

func (h *Handler) addRecord(ticketID, actorID uint, action string, from, to model.TicketStatus, remark string) {
	_ = h.db.Create(&model.TicketRecord{
		TicketID:   ticketID,
		ActorID:    actorID,
		Action:     action,
		FromStatus: from,
		ToStatus:   to,
		Remark:     remark,
	}).Error
}

func (h *Handler) notifyCurrentApprovers(ticketID uint) {
	mailConfig := h.currentMailConfig()
	if mailConfig.ID == 0 || !mailConfig.Enabled {
		return
	}
	password, err := h.mailConfigPassword(mailConfig)
	if err != nil {
		h.addSystemRecord(ticketID, "mail_failed", "SMTP 密码解密失败")
		return
	}
	var ticket model.Ticket
	if err := h.db.Preload("Applicant").Preload("Asset").First(&ticket, ticketID).Error; err != nil {
		return
	}
	if ticket.CurrentWorkflowStepID == nil {
		return
	}
	var step model.TicketWorkflowStep
	if err := h.db.Preload("Approvers.User").First(&step, *ticket.CurrentWorkflowStepID).Error; err != nil {
		return
	}
	recipients := make([]mail.Address, 0, len(step.Approvers))
	for _, approver := range step.Approvers {
		address, err := mail.ParseAddress(strings.TrimSpace(approver.User.Email))
		if err != nil {
			continue
		}
		if approver.User.Name != "" {
			address.Name = approver.User.Name
		}
		recipients = append(recipients, *address)
	}
	if len(recipients) == 0 {
		h.addSystemRecord(ticketID, "mail_skipped", "当前审批节点没有可用审批人邮箱")
		return
	}
	message := service.MailMessage{
		To:      recipients,
		Subject: fmt.Sprintf("待审批工单 #%d：%s", ticket.ID, ticket.Title),
		Body:    approvalMailBody(ticket, step),
	}
	if err := h.mail.Send(mailConfig, password, message); err != nil {
		h.addSystemRecord(ticketID, "mail_failed", "审批通知邮件发送失败: "+err.Error())
		return
	}
	h.addSystemRecord(ticketID, "mail_sent", fmt.Sprintf("已通知审批节点 %s，共 %d 人", step.Name, len(recipients)))
}

func (h *Handler) addSystemRecord(ticketID uint, action, remark string) {
	var ticket model.Ticket
	if err := h.db.First(&ticket, ticketID).Error; err != nil {
		return
	}
	h.addRecord(ticketID, ticket.ApplicantID, action, ticket.Status, ticket.Status, remark)
}

func approvalMailBody(ticket model.Ticket, step model.TicketWorkflowStep) string {
	lines := []string{
		"您好，",
		"",
		"有一张工单等待您审批。",
		"",
		fmt.Sprintf("工单编号：#%d", ticket.ID),
		"工单标题：" + ticket.Title,
		"审批节点：" + step.Name,
		"工单类型：" + string(ticket.Type),
		"优先级：" + string(ticket.Priority),
		"申请人：" + defaultString(ticket.Applicant.Name, ticket.Applicant.Username),
	}
	if ticket.Asset != nil {
		lines = append(lines, "关联资产："+defaultString(ticket.Asset.Hostname, ticket.Asset.IP))
	}
	if strings.TrimSpace(ticket.Description) != "" {
		lines = append(lines, "", "申请说明：", ticket.Description)
	}
	lines = append(lines, "", "请登录资产管理系统处理。")
	return strings.Join(lines, "\n")
}

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
