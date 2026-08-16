package httpapi

import (
	"net/http"

	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"

	"github.com/gin-gonic/gin"
)

// credentialRequest 凭据请求体（secret 为明文，仅创建/更新时提交）。
type credentialRequest struct {
	AssetID  *uint  `json:"assetId"`
	Name     string `json:"name" binding:"required"`
	Username string `json:"username" binding:"required"`
	Type     string `json:"type"`
	Secret   string `json:"secret"`
	Remark   string `json:"remark"`
}

func (h *Handler) credentialService() *service.CredentialService {
	return service.NewCredentialService(h.db, h.cfg.Security.ConfigEncryptionKey)
}

func normalizeCredentialType(t string) string {
	if t == "" {
		return "ssh"
	}
	return t
}

// ListCredentials 列出全部凭据（不含明文）。
// @Summary 凭据列表
// @Description 查询全部凭据（admin/asset_manager），不含明文
// @Tags credentials
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /credentials [get]
func (h *Handler) ListCredentials(c *gin.Context) {
	list, err := h.credentialService().List()
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询凭据失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list})
}

// CreateCredential 创建凭据（加密存储 secret）。
// @Summary 创建凭据
// @Description 创建凭据，secret 以 AES-GCM 加密存储（admin/asset_manager）
// @Tags credentials
// @Accept json
// @Produce json
// @Param body body credentialRequest true "凭据"
// @Success 201 {object} model.Credential
// @Failure 400 {object} errorResponse
// @Security BearerAuth
// @Router /credentials [post]
func (h *Handler) CreateCredential(c *gin.Context) {
	var req credentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数无效")
		return
	}
	if req.Secret == "" {
		errorJSON(c, http.StatusBadRequest, "secret 不能为空")
		return
	}
	cred := &model.Credential{
		AssetID:  req.AssetID,
		Name:     req.Name,
		Username: req.Username,
		Type:     normalizeCredentialType(req.Type),
		Remark:   req.Remark,
	}
	if err := h.credentialService().Create(cred, req.Secret); err != nil {
		errorJSON(c, http.StatusInternalServerError, "创建凭据失败")
		return
	}
	h.audit(currentUser(c).ID, "credential", cred.ID, "create", cred.Name)
	c.JSON(http.StatusCreated, cred)
}

// UpdateCredential 更新凭据（secret 非空时重新加密）。
// @Summary 更新凭据
// @Description 更新凭据元信息，secret 非空时重新加密存储（admin/asset_manager）
// @Tags credentials
// @Accept json
// @Produce json
// @Param id path int true "凭据 ID"
// @Param body body credentialRequest true "凭据"
// @Success 200 {object} model.Credential
// @Failure 400 {object} errorResponse
// @Security BearerAuth
// @Router /credentials/{id} [put]
func (h *Handler) UpdateCredential(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req credentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数无效")
		return
	}
	cred := &model.Credential{
		AssetID:  req.AssetID,
		Name:     req.Name,
		Username: req.Username,
		Type:     normalizeCredentialType(req.Type),
		Remark:   req.Remark,
	}
	if err := h.credentialService().Update(id, cred, req.Secret); err != nil {
		errorJSON(c, http.StatusInternalServerError, "更新凭据失败")
		return
	}
	h.audit(currentUser(c).ID, "credential", id, "update", cred.Name)
	c.JSON(http.StatusOK, cred)
}

// DeleteCredential 删除凭据。
// @Summary 删除凭据
// @Description 删除凭据（admin/asset_manager）
// @Tags credentials
// @Produce json
// @Param id path int true "凭据 ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errorResponse
// @Security BearerAuth
// @Router /credentials/{id} [delete]
func (h *Handler) DeleteCredential(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.credentialService().Delete(id); err != nil {
		errorJSON(c, http.StatusInternalServerError, "删除凭据失败")
		return
	}
	h.audit(currentUser(c).ID, "credential", id, "delete", "")
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// RevealCredential 解密返回明文 secret（写审计并更新最后访问时间）。
// @Summary 查看凭据明文
// @Description 解密返回明文 secret，写入查看审计（admin/asset_manager）
// @Tags credentials
// @Produce json
// @Param id path int true "凭据 ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errorResponse
// @Security BearerAuth
// @Router /credentials/{id}/reveal [post]
func (h *Handler) RevealCredential(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	cred, secret, err := h.credentialService().Reveal(id)
	if err != nil {
		errorJSON(c, http.StatusNotFound, "凭据不存在或解密失败")
		return
	}
	h.audit(currentUser(c).ID, "credential", id, "reveal", cred.Name)
	c.JSON(http.StatusOK, gin.H{"credential": cred, "secret": secret})
}
