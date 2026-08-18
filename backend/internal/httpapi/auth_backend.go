package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
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
