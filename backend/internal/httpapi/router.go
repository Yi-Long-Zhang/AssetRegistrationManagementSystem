package httpapi

import (
	"net/http"
	"strings"
	"time"

	_ "asset-registration-management-system/backend/docs"
	"asset-registration-management-system/backend/internal/config"
	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
)

type Dependencies struct {
	Config   config.Config
	DB       *gorm.DB
	Roles    []model.Role
	AD       service.ADClient
	Archiver service.TicketArchiver
	Mail     service.MailSender
}

func NewRouter(dep Dependencies) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), cors(dep.Config.CORS.AllowedOrigins))

	adClient := dep.AD
	if adClient == nil {
		adClient = service.LDAPADClient{}
	}
	archiver := dep.Archiver
	if archiver == nil {
		archiver = service.LibreOfficeTicketArchiver{}
	}
	mailSender := dep.Mail
	if mailSender == nil {
		mailSender = service.SMTPMailSender{}
	}
	h := NewHandler(dep.Config, dep.DB, dep.Roles, adClient, archiver, mailSender)

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now().UTC()})
	})
	if dep.Config.Swagger.Enabled {
		router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	api := router.Group("/api/v1")
	api.POST("/auth/login", h.Login)
	api.POST("/auth/logout", h.Logout)
	api.GET("/auth/me", h.AuthRequired(), h.Me)
	api.POST("/auth/change-password", h.AuthRequired(), h.ChangePassword)
	api.POST("/im/callback", h.IMCallback)

	api.GET("/roles", h.AuthRequired(), h.RequireAnyRole(model.RoleAdmin), h.ListRoles)

	ad := api.Group("/ad", h.AuthRequired(), h.RequireAnyRole(model.RoleAdmin))
	ad.GET("/config", h.GetADConfig)
	ad.PUT("/config", h.SaveADConfig)
	ad.POST("/test", h.TestADConnection)
	ad.POST("/lookup-user", h.LookupADUser)
	ad.POST("/import-user", h.ImportADUser)

	settings := api.Group("/settings", h.AuthRequired(), h.RequireAnyRole(model.RoleAdmin))
	auditLogs := api.Group("/audit-logs", h.AuthRequired(), h.RequireAnyRole(model.RoleAdmin))
	auditLogs.GET("", h.ListAuditLogs)

	backups := api.Group("/backups", h.AuthRequired(), h.RequireAnyRole(model.RoleAdmin))
	backups.GET("", h.ListBackups)
	backups.POST("", h.CreateBackup)
	backups.DELETE("/:name", h.DeleteBackup)
	backups.GET("/:name/download", h.DownloadBackup)
	backups.POST("/:name/restore", h.RestoreBackup)
	settings.GET("/mail", h.GetMailConfig)
	settings.PUT("/mail", h.SaveMailConfig)
	settings.GET("/im", h.GetIMConfig)
	settings.PUT("/im", h.SaveIMConfig)
	settings.POST("/im/test", h.TestIMConfig)
	settings.GET("/im/callback", h.GetIMCallbackConfig)
	settings.PUT("/im/callback", h.SaveIMCallbackConfig)
	settings.GET("/im/bindings", h.ListIMBindings)
	settings.PUT("/im/bindings", h.SaveIMBinding)
	settings.DELETE("/im/bindings/:userId", h.DeleteIMBinding)
	settings.POST("/mail/test", h.TestMailConfig)

	users := api.Group("/users", h.AuthRequired(), h.RequireAnyRole(model.RoleAdmin))
	users.GET("", h.ListUsers)
	users.POST("", h.CreateUser)
	users.PUT("/:id", h.UpdateUser)

	typeApprovers := api.Group("/ticket-type-approvers", h.AuthRequired(), h.RequireAnyRole(model.RoleAdmin))
	typeApprovers.GET("", h.ListTicketTypeApprovers)
	typeApprovers.PUT("/:type", h.SetTicketTypeApprover)

	workflows := api.Group("/workflows", h.AuthRequired(), h.RequireAnyRole(model.RoleAdmin))
	workflows.GET("", h.ListWorkflows)
	workflows.GET("/:type", h.GetWorkflow)
	workflows.PUT("/:type", h.SaveWorkflow)
	workflows.POST("/:type/enable", h.EnableWorkflow)

	assets := api.Group("/assets", h.AuthRequired())
	assets.GET("", h.ListAssets)
	assets.GET("/stats", h.AssetStats)
	assets.GET("/stats/export", h.RequireAnyRole(model.RoleAdmin, model.RoleAssetManager), h.ExportAssetStats)
	assets.POST("", h.RequireAnyRole(model.RoleAdmin, model.RoleAssetManager), h.CreateAsset)
	assets.GET("/export", h.RequireAnyRole(model.RoleAdmin, model.RoleAssetManager), h.ExportAssets)
	assets.GET("/template", h.RequireAnyRole(model.RoleAdmin, model.RoleAssetManager), h.DownloadAssetImportTemplate)
	assets.POST("/import", h.RequireAnyRole(model.RoleAdmin, model.RoleAssetManager), h.ImportAssets)
	assets.GET("/:id", h.GetAsset)
	assets.GET("/:id/history", h.ListAssetHistory)
	assets.PUT("/:id", h.RequireAnyRole(model.RoleAdmin, model.RoleAssetManager), h.UpdateAsset)
	assets.DELETE("/:id", h.RequireAnyRole(model.RoleAdmin, model.RoleAssetManager), h.DeleteAsset)
	assets.POST("/:id/retire", h.RequireAnyRole(model.RoleAdmin, model.RoleAssetManager), h.RetireAsset)
	assets.POST("/batch-delete", h.RequireAnyRole(model.RoleAdmin, model.RoleAssetManager), h.BatchDeleteAssets)
	assets.POST("/batch-update", h.RequireAnyRole(model.RoleAdmin, model.RoleAssetManager), h.BatchUpdateAssets)

	discovery := api.Group("/discovery", h.AuthRequired(), h.RequireAnyRole(model.RoleAdmin, model.RoleAssetManager))
	discovery.GET("/rules", h.ListDiscoveryRules)
	discovery.POST("/rules", h.CreateDiscoveryRule)
	discovery.PUT("/rules/:id", h.UpdateDiscoveryRule)
	discovery.DELETE("/rules/:id", h.DeleteDiscoveryRule)
	discovery.POST("/rules/:id/run", h.StartDiscoveryRun)
	discovery.POST("/rules/:id/test", h.TestDiscoveryRun)
	discovery.GET("/runs", h.ListDiscoveryRuns)
	discovery.GET("/runs/:id", h.GetDiscoveryRun)
	discovery.POST("/runs/:id/adopt", h.AdoptDiscoveryHosts)

	inspection := api.Group("/inspection", h.AuthRequired(), h.RequireAnyRole(model.RoleAdmin))
	inspection.GET("/rules", h.ListInspectionRules)
	inspection.POST("/rules", h.CreateInspectionRule)
	inspection.PUT("/rules/:id", h.UpdateInspectionRule)
	inspection.DELETE("/rules/:id", h.DeleteInspectionRule)
	inspection.POST("/rules/:id/test", h.TestInspectionRule)

	stocktakes := api.Group("/stocktakes", h.AuthRequired(), h.RequireAnyRole(model.RoleAdmin, model.RoleAssetManager))
	stocktakes.GET("", h.ListStocktakes)
	stocktakes.POST("", h.CreateStocktake)
	stocktakes.GET("/:id", h.GetStocktake)
	stocktakes.PUT("/:id/items/:itemId", h.UpdateStocktakeItem)
	stocktakes.POST("/:id/close", h.CloseStocktake)
	stocktakes.GET("/:id/export", h.ExportStocktake)

	rooms := api.Group("/rooms", h.AuthRequired(), h.RequireAnyRole(model.RoleAdmin, model.RoleAssetManager))
	rooms.GET("", h.ListRooms)
	rooms.POST("", h.CreateRoom)
	rooms.PUT("/:id", h.UpdateRoom)
	rooms.DELETE("/:id", h.DeleteRoom)

	racks := api.Group("/racks", h.AuthRequired(), h.RequireAnyRole(model.RoleAdmin, model.RoleAssetManager))
	racks.GET("", h.ListRacks)
	racks.POST("", h.CreateRack)
	racks.PUT("/:id", h.UpdateRack)
	racks.DELETE("/:id", h.DeleteRack)

	ipSegments := api.Group("/ip-segments", h.AuthRequired(), h.RequireAnyRole(model.RoleAdmin, model.RoleAssetManager))
	ipSegments.GET("", h.ListIPSegments)
	ipSegments.POST("", h.CreateIPSegment)
	ipSegments.PUT("/:id", h.UpdateIPSegment)
	ipSegments.DELETE("/:id", h.DeleteIPSegment)
	ipSegments.GET("/:id/usage", h.GetIPSegmentUsage)
	discovery.POST("/runs/:id/apply", h.ApplyDiscoveryHosts)
	discovery.GET("/stats/trend", h.GetDiscoveryTrend)
	discovery.GET("/stats/subnets", h.GetDiscoverySubnetStats)
	discovery.GET("/stats/services", h.GetDiscoveryServiceStats)

	tickets := api.Group("/tickets", h.AuthRequired())
	tickets.GET("", h.ListTickets)
	tickets.POST("", h.CreateTicket)
	tickets.GET("/stats", h.RequireAnyRole(model.RoleAdmin), h.GetTicketStats)
	tickets.GET("/stats/export", h.RequireAnyRole(model.RoleAdmin), h.ExportTicketStats)
	tickets.POST("/archives/download", h.DownloadTicketArchives)
	tickets.POST("/batch-approve", h.BatchApproveTickets)
	tickets.GET("/:id", h.GetTicket)
	tickets.PUT("/:id", h.UpdateTicket)
	tickets.GET("/:id/comments", h.ListTicketComments)
	tickets.POST("/:id/comments", h.CreateTicketComment)
	tickets.GET("/:id/attachments", h.ListTicketAttachments)
	tickets.POST("/:id/attachments", h.UploadTicketAttachment)
	tickets.GET("/:id/attachments/:attachmentId/download", h.DownloadTicketAttachment)
	tickets.GET("/:id/archive/download", h.DownloadTicketArchive)
	tickets.POST("/:id/transfer", h.TransferTicketApprover)
	for _, action := range []string{"submit", "withdraw", "approve", "reject", "start", "complete", "accept", "cancel"} {
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
