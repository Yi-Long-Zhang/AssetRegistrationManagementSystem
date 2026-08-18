package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

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
