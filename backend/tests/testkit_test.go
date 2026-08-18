package tests

import (
	"net/http"
	"path/filepath"
	"testing"

	"asset-registration-management-system/backend/internal/config"
	"asset-registration-management-system/backend/internal/database"
	"asset-registration-management-system/backend/internal/httpapi"
	"asset-registration-management-system/backend/internal/model"
)

func testRouter(t *testing.T) http.Handler {
	t.Helper()
	return testRouterWithConfig(t, config.Config{})
}

func testRouterWithConfig(t *testing.T, cfgOverride config.Config) http.Handler {
	t.Helper()
	dir := t.TempDir()
	db, err := database.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := database.SeedAdmin(db, "admin", "admin123456"); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.App.Env = config.EnvTest
	cfg.HTTP.Addr = ":0"
	cfg.Storage.DatabasePath = filepath.Join(dir, "test.db")
	cfg.Storage.AttachmentDir = filepath.Join(dir, "attachments")
	cfg.Storage.TicketArchiveDir = filepath.Join(dir, "archives")
	cfg.Storage.BackupDir = filepath.Join(dir, "backups")
	cfg.Security.JWTSecret = "test-secret"
	cfg.Security.ConfigEncryptionKey = "test-config-key"
	cfg.Auth.Mode = "mixed"
	cfg.CORS.AllowedOrigins = "*"
	mergeConfig(&cfg, cfgOverride)

	return httpapi.NewRouter(httpapi.Dependencies{
		Config:   cfg,
		DB:       db,
		Roles:    model.AllRoles(),
		AD:       fakeADClient{},
		Archiver: fakeArchiver{},
		Mail:     fakeMailSender{},
	})
}

func mergeConfig(target *config.Config, override config.Config) {
	if override.App.Env != "" {
		target.App.Env = override.App.Env
	}
	if override.HTTP.Addr != "" {
		target.HTTP.Addr = override.HTTP.Addr
	}
	if override.Storage.DatabasePath != "" {
		target.Storage.DatabasePath = override.Storage.DatabasePath
	}
	if override.Storage.AttachmentDir != "" {
		target.Storage.AttachmentDir = override.Storage.AttachmentDir
	}
	if override.Storage.TicketArchiveDir != "" {
		target.Storage.TicketArchiveDir = override.Storage.TicketArchiveDir
	}
	if override.Storage.TicketTemplatePath != "" {
		target.Storage.TicketTemplatePath = override.Storage.TicketTemplatePath
	}
	if override.Storage.LibreOfficeBin != "" {
		target.Storage.LibreOfficeBin = override.Storage.LibreOfficeBin
	}
	if override.Security.JWTSecret != "" {
		target.Security.JWTSecret = override.Security.JWTSecret
	}
	if override.Security.ConfigEncryptionKey != "" {
		target.Security.ConfigEncryptionKey = override.Security.ConfigEncryptionKey
	}
	if len(override.Security.JWTPreviousSecrets) > 0 {
		target.Security.JWTPreviousSecrets = override.Security.JWTPreviousSecrets
	}
	if override.Security.LoginMaxAttempts > 0 {
		target.Security.LoginMaxAttempts = override.Security.LoginMaxAttempts
	}
	if override.Security.LoginWindowMinutes > 0 {
		target.Security.LoginWindowMinutes = override.Security.LoginWindowMinutes
	}
	if override.Security.LoginBlockMinutes > 0 {
		target.Security.LoginBlockMinutes = override.Security.LoginBlockMinutes
	}
	if override.Security.SensitiveReauthMinutes > 0 {
		target.Security.SensitiveReauthMinutes = override.Security.SensitiveReauthMinutes
	}
	if override.Auth.Mode != "" {
		target.Auth.Mode = override.Auth.Mode
	}
	if override.TokenTTL != 0 {
		target.TokenTTL = override.TokenTTL
	}
	if override.Admin.Username != "" {
		target.Admin.Username = override.Admin.Username
	}
	if override.Admin.Password != "" {
		target.Admin.Password = override.Admin.Password
	}
	if override.CORS.AllowedOrigins != "" {
		target.CORS.AllowedOrigins = override.CORS.AllowedOrigins
	}
	target.Swagger.Enabled = override.Swagger.Enabled
}
