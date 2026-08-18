package httpapi

import (
	"net/http"

	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"

	"github.com/gin-gonic/gin"
)

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
