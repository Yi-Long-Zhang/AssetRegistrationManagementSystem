package httpapi

import (
	"time"

	"asset-registration-management-system/backend/internal/config"
	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

type Handler struct {
	cfg      config.Config
	db       *gorm.DB
	roles    []model.Role
	ad       service.ADClient
	archiver service.TicketArchiver
	mail     service.MailSender
	im       service.IMNotifier // nil 时使用默认群机器人通知器
	limiter  *loginRateLimiter
	tasks    *service.TaskManager
	metrics  *HTTPMetrics
}

type claims struct {
	UserID         uint       `json:"userId"`
	Role           model.Role `json:"role"`
	SessionID      string     `json:"sid"`
	SessionVersion uint64     `json:"sv"`
	jwt.RegisteredClaims
}

func NewHandler(cfg config.Config, db *gorm.DB, roles []model.Role, ad service.ADClient, archiver service.TicketArchiver, mailSender service.MailSender) *Handler {
	return &Handler{
		cfg: cfg, db: db, roles: roles, ad: ad, archiver: archiver, mail: mailSender,
		limiter: newLoginRateLimiter(
			cfg.Security.LoginMaxAttempts,
			time.Duration(cfg.Security.LoginWindowMinutes)*time.Minute,
			time.Duration(cfg.Security.LoginBlockMinutes)*time.Minute,
		),
	}
}
