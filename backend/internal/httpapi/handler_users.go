package httpapi

import (
	"net/http"

	"asset-registration-management-system/backend/internal/model"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func (h *Handler) ListUsers(c *gin.Context) {
	var users []model.User
	if err := h.db.Order("id desc").Find(&users).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询用户失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": users})
}

func (h *Handler) CreateUser(c *gin.Context) {
	var req userRequest
	if !bind(c, &req) {
		return
	}
	authSource := defaultString(req.AuthSource, "local")
	if authSource == "ad" {
		errorJSON(c, http.StatusBadRequest, "请通过 AD 导入接口创建 AD 用户")
		return
	}
	if req.Password == "" {
		errorJSON(c, http.StatusBadRequest, "密码不能为空")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "生成密码失败")
		return
	}
	user := model.User{
		Username:     req.Username,
		Name:         req.Name,
		DisplayName:  req.DisplayName,
		Email:        req.Email,
		Department:   req.Department,
		Role:         req.Role,
		Status:       defaultString(req.Status, "active"),
		AuthSource:   authSource,
		ProxyUserID:  req.ProxyUserID,
		PasswordHash: string(hash),
	}
	if err := h.db.Create(&user).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "创建用户失败: "+err.Error())
		return
	}
	c.JSON(http.StatusCreated, user)
}

func (h *Handler) UpdateUser(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var user model.User
	if !h.findByID(c, id, &user) {
		return
	}
	var req userRequest
	if !bind(c, &req) {
		return
	}
	previousRole := user.Role
	previousStatus := user.Status
	user.Username = req.Username
	user.Name = req.Name
	user.DisplayName = req.DisplayName
	user.Email = req.Email
	user.Department = req.Department
	user.Role = req.Role
	user.Status = defaultString(req.Status, "active")
	user.ProxyUserID = req.ProxyUserID
	if user.AuthSource == "ad" && req.Password != "" {
		errorJSON(c, http.StatusBadRequest, "AD 用户不能在本系统修改密码")
		return
	}
	if user.AuthSource != "ad" && req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			errorJSON(c, http.StatusInternalServerError, "生成密码失败")
			return
		}
		user.PasswordHash = string(hash)
	}
	invalidateSessions := req.Password != "" || previousRole != user.Role || previousStatus != user.Status
	if invalidateSessions {
		user.SessionVersion++
	}
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&user).Error; err != nil {
			return err
		}
		if invalidateSessions {
			return revokeUserSessions(tx, user.ID, "user_updated", "")
		}
		return nil
	}); err != nil {
		errorJSON(c, http.StatusBadRequest, "更新用户失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, user)
}
