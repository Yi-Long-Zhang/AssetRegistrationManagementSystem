package httpapi

import (
	"log"
	"net/http"
	"strings"

	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"

	"github.com/gin-gonic/gin"
)

// imCallbackConfigRequest IM 回调验签配置请求
type imCallbackConfigRequest struct {
	Enabled        bool   `json:"enabled"`
	Platform       string `json:"platform"`
	AppSecret      string `json:"appSecret"`
	CorpID         string `json:"corpId"`
	Token          string `json:"token"`
	EncodingAESKey string `json:"encodingAESKey"`
}

// GetIMCallbackConfig 读取 IM 回调验签配置（admin，敏感字段不回显）。
// @Summary IM 回调验签配置
// @Description 读取钉钉/企微/飞书自建应用回调验签配置（敏感字段不回显）
// @Tags settings
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /settings/im/callback [get]
// @Security BearerAuth
func (h *Handler) GetIMCallbackConfig(c *gin.Context) {
	cb := h.currentIMCallbackConfig()
	c.JSON(http.StatusOK, gin.H{
		"id":                cb.ID,
		"enabled":           cb.Enabled,
		"platform":          cb.Platform,
		"corpId":            cb.CorpID,
		"hasAppSecret":      cb.AppSecret != "",
		"hasToken":          cb.Token != "",
		"hasEncodingAESKey": cb.EncodingAESKey != "",
	})
}

// SaveIMCallbackConfig 保存 IM 回调验签配置（admin，敏感字段 AES-GCM 加密存储）。
// @Summary 保存 IM 回调验签配置
// @Description 保存钉钉/企微/飞书自建应用回调验签配置（敏感字段加密存储，留空不修改）
// @Tags settings
// @Accept json
// @Produce json
// @Param body body imCallbackConfigRequest true "回调验签配置"
// @Success 200 {object} map[string]interface{}
// @Router /settings/im/callback [put]
// @Security BearerAuth
func (h *Handler) SaveIMCallbackConfig(c *gin.Context) {
	var req imCallbackConfigRequest
	if !bind(c, &req) {
		return
	}
	cb := h.currentIMCallbackConfig()
	cb.Enabled = req.Enabled
	cb.Platform = defaultString(strings.TrimSpace(req.Platform), string(service.IMPlatformDingTalk))
	cb.CorpID = strings.TrimSpace(req.CorpID)
	if err := h.encryptCallbackField(&cb, &cb.AppSecret, req.AppSecret, "app_secret"); err != nil {
		errorJSON(c, http.StatusBadRequest, "加密 AppSecret 失败: "+err.Error())
		return
	}
	if err := h.encryptCallbackField(&cb, &cb.Token, req.Token, "token"); err != nil {
		errorJSON(c, http.StatusBadRequest, "加密 Token 失败: "+err.Error())
		return
	}
	if err := h.encryptCallbackField(&cb, &cb.EncodingAESKey, req.EncodingAESKey, "aes_key"); err != nil {
		errorJSON(c, http.StatusBadRequest, "加密 EncodingAESKey 失败: "+err.Error())
		return
	}
	if err := h.db.Save(&cb).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "保存 IM 回调配置失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id": cb.ID, "enabled": cb.Enabled, "platform": cb.Platform, "corpId": cb.CorpID,
		"hasAppSecret": cb.AppSecret != "", "hasToken": cb.Token != "", "hasEncodingAESKey": cb.EncodingAESKey != "",
	})
}

// encryptCallbackField 非空时加密写入；留空表示不修改既有值。
func (h *Handler) encryptCallbackField(cb *model.IMCallbackConfig, dst *string, value, name string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	encrypted, err := service.EncryptString(value, h.cfg.Security.ConfigEncryptionKey)
	if err != nil {
		return err
	}
	*dst = encrypted
	cb.Encrypted = true
	return nil
}

// currentIMCallbackConfig 读取 IM 回调配置（不存在时返回默认关闭配置）。
func (h *Handler) currentIMCallbackConfig() model.IMCallbackConfig {
	var cb model.IMCallbackConfig
	if err := h.db.First(&cb).Error; err != nil {
		return model.IMCallbackConfig{Platform: string(service.IMPlatformDingTalk)}
	}
	return cb
}

// imCallbackSecret 解密回调配置中的敏感字段；未加密或解密失败时返回原值。
// field: app_secret / token
func (h *Handler) imCallbackSecret(cb model.IMCallbackConfig, field string) string {
	var cipherText string
	switch field {
	case "app_secret":
		cipherText = cb.AppSecret
	case "token":
		cipherText = cb.Token
	case "aes_key":
		cipherText = cb.EncodingAESKey
	}
	if cipherText == "" {
		return ""
	}
	if !cb.Encrypted {
		return cipherText
	}
	dec, err := service.DecryptString(cipherText, h.cfg.Security.ConfigEncryptionKey)
	if err != nil {
		log.Printf("im callback: decrypt %s: %v", field, err)
		return ""
	}
	return dec
}
