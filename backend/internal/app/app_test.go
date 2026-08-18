package app

import (
	"path/filepath"
	"testing"

	"asset-registration-management-system/backend/internal/config"
)

func TestNewAndClose(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.App.Env = config.EnvTest
	cfg.HTTP.Addr = "127.0.0.1:0"
	cfg.Storage.DatabasePath = filepath.Join(dir, "data", "assets.db")
	cfg.Storage.AttachmentDir = filepath.Join(dir, "data", "attachments")
	cfg.Storage.TicketArchiveDir = filepath.Join(dir, "data", "archives")
	cfg.Storage.BackupDir = filepath.Join(dir, "data", "backups")
	cfg.Security.JWTSecret = "test-jwt-secret"
	cfg.Security.ConfigEncryptionKey = "test-config-key"
	cfg.Admin.Username = "admin"
	cfg.Admin.Password = "admin123456"

	application, err := New(cfg)
	if err != nil {
		t.Fatalf("create application: %v", err)
	}
	if err := application.Close(); err != nil {
		t.Fatalf("close application: %v", err)
	}
	if err := application.Close(); err != nil {
		t.Fatalf("close application twice: %v", err)
	}
}
