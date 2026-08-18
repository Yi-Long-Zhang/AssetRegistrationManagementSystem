package httpapi

import (
	"net/http"

	"asset-registration-management-system/backend/internal/model"

	"github.com/gin-gonic/gin"
)

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
