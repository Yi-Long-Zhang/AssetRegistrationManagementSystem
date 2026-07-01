package config

import (
	"os"
	"time"
)

type Config struct {
	HTTPAddr       string
	DatabasePath   string
	AttachmentDir  string
	ArchiveDir     string
	TicketTemplate string
	LibreOfficeBin string
	JWTSecret      string
	AuthMode       string
	ConfigKey      string
	TokenTTL       time.Duration
	AdminUsername  string
	AdminPassword  string
	AllowedOrigins string
}

func Load() Config {
	return Config{
		HTTPAddr:       env("HTTP_ADDR", ":8080"),
		DatabasePath:   env("DATABASE_PATH", "data/assets.db"),
		AttachmentDir:  env("ATTACHMENT_DIR", "data/attachments"),
		ArchiveDir:     env("TICKET_ARCHIVE_DIR", "data/ticket-archives"),
		TicketTemplate: env("TICKET_TEMPLATE_PATH", "../templates/ticket-it-change-template.docx"),
		LibreOfficeBin: env("LIBREOFFICE_BIN", "soffice"),
		JWTSecret:      env("JWT_SECRET", "change-me-in-production"),
		AuthMode:       env("AUTH_MODE", "mixed"),
		ConfigKey:      env("CONFIG_ENCRYPTION_KEY", env("APP_SECRET_KEY", "change-me-config-key")),
		TokenTTL:       24 * time.Hour,
		AdminUsername:  env("ADMIN_USERNAME", "admin"),
		AdminPassword:  env("ADMIN_PASSWORD", "admin123456"),
		AllowedOrigins: env("ALLOWED_ORIGINS", "*"),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
