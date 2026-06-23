package httpapi

import (
	"net/http"
	"strings"
	"time"

	"asset-registration-management-system/backend/internal/config"
	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Dependencies struct {
	Config config.Config
	DB     *gorm.DB
	Roles  []model.Role
	AD     service.ADClient
}

func NewRouter(dep Dependencies) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), cors(dep.Config.AllowedOrigins))

	adClient := dep.AD
	if adClient == nil {
		adClient = service.LDAPADClient{}
	}
	h := NewHandler(dep.Config, dep.DB, dep.Roles, adClient)

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now().UTC()})
	})
	router.GET("/swagger/index.html", h.SwaggerIndex)
	router.GET("/swagger/doc.json", h.OpenAPISpec)

	api := router.Group("/api/v1")
	api.POST("/auth/login", h.Login)
	api.POST("/auth/logout", h.Logout)
	api.GET("/auth/me", h.AuthRequired(), h.Me)

	api.GET("/roles", h.AuthRequired(), h.RequireAnyRole(model.RoleAdmin), h.ListRoles)

	ad := api.Group("/ad", h.AuthRequired(), h.RequireAnyRole(model.RoleAdmin))
	ad.GET("/config", h.GetADConfig)
	ad.PUT("/config", h.SaveADConfig)
	ad.POST("/test", h.TestADConnection)
	ad.POST("/lookup-user", h.LookupADUser)
	ad.POST("/import-user", h.ImportADUser)

	users := api.Group("/users", h.AuthRequired(), h.RequireAnyRole(model.RoleAdmin))
	users.GET("", h.ListUsers)
	users.POST("", h.CreateUser)
	users.PUT("/:id", h.UpdateUser)

	typeApprovers := api.Group("/ticket-type-approvers", h.AuthRequired(), h.RequireAnyRole(model.RoleAdmin))
	typeApprovers.GET("", h.ListTicketTypeApprovers)
	typeApprovers.PUT("/:type", h.SetTicketTypeApprover)

	assets := api.Group("/assets", h.AuthRequired())
	assets.GET("", h.ListAssets)
	assets.POST("", h.RequireAnyRole(model.RoleAdmin, model.RoleAssetManager), h.CreateAsset)
	assets.GET("/:id", h.GetAsset)
	assets.PUT("/:id", h.RequireAnyRole(model.RoleAdmin, model.RoleAssetManager), h.UpdateAsset)
	assets.DELETE("/:id", h.RequireAnyRole(model.RoleAdmin, model.RoleAssetManager), h.DeleteAsset)

	tickets := api.Group("/tickets", h.AuthRequired())
	tickets.GET("", h.ListTickets)
	tickets.POST("", h.CreateTicket)
	tickets.GET("/:id", h.GetTicket)
	tickets.PUT("/:id", h.UpdateTicket)
	tickets.GET("/:id/comments", h.ListTicketComments)
	tickets.POST("/:id/comments", h.CreateTicketComment)
	tickets.GET("/:id/attachments", h.ListTicketAttachments)
	tickets.POST("/:id/attachments", h.UploadTicketAttachment)
	tickets.GET("/:id/attachments/:attachmentId/download", h.DownloadTicketAttachment)
	for _, action := range []string{"submit", "approve", "reject", "start", "complete", "close", "cancel"} {
		tickets.POST("/:id/"+action, h.TicketAction(action))
	}

	return router
}

func cors(allowedOrigins string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		allow := allowedOrigins == "*" || originAllowed(origin, allowedOrigins)
		if allow {
			if allowedOrigins == "*" {
				c.Header("Access-Control-Allow-Origin", "*")
			} else {
				c.Header("Access-Control-Allow-Origin", origin)
			}
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func originAllowed(origin, allowedOrigins string) bool {
	for _, item := range strings.Split(allowedOrigins, ",") {
		if strings.TrimSpace(item) == origin {
			return true
		}
	}
	return false
}
