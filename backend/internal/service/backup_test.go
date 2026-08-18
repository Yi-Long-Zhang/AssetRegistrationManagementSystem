package service_test

import (
	"os"
	"path/filepath"
	"testing"

	"asset-registration-management-system/backend/internal/config"
	"asset-registration-management-system/backend/internal/database"
	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"
)

func TestFullBackupRestoreIncludesFilesAndConfig(t *testing.T) {
	root := t.TempDir()
	cfg := config.Default()
	cfg.App.Env = config.EnvTest
	cfg.Storage.DatabasePath = filepath.Join(root, "data", "assets.db")
	cfg.Storage.AttachmentDir = filepath.Join(root, "data", "attachments")
	cfg.Storage.TicketArchiveDir = filepath.Join(root, "data", "archives")
	cfg.Storage.BackupDir = filepath.Join(root, "backups")
	cfg.Storage.BackupOffsiteDir = filepath.Join(root, "offsite")
	cfg.ConfigPath = filepath.Join(root, "config.yaml")
	cfg.Security.ConfigEncryptionKey = "backup-test-key"

	db, err := database.Open(cfg.Storage.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Asset{AssetNo: "A-1", Hostname: "server-1", IP: "10.0.0.1", Status: model.AssetStatusInUse}).Error; err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(cfg.Storage.AttachmentDir, "ticket.txt"), "attachment")
	mustWrite(t, filepath.Join(cfg.Storage.TicketArchiveDir, "ticket.pdf"), "archive")
	mustWrite(t, cfg.ConfigPath, "app:\n  env: test\n")

	backup := service.NewFullBackupService(db, cfg)
	info, err := backup.Create()
	if err != nil {
		t.Fatal(err)
	}
	if !info.Encrypted || !info.OffsiteCopied || info.SHA256 == "" || len(info.Contents) < 4 {
		t.Fatalf("unexpected backup metadata: %+v", info)
	}
	if _, err := backup.Verify(info.Name); err != nil {
		t.Fatal(err)
	}
	if err := backup.Restore(info.Name); err != nil {
		t.Fatal(err)
	}

	sqlDB, _ := db.DB()
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(cfg.Storage.AttachmentDir); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(cfg.Storage.TicketArchiveDir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.ConfigPath, []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if restored, err := backup.ApplyPendingRestore(); err != nil || !restored {
		t.Fatalf("apply restore restored=%v err=%v", restored, err)
	}
	assertFile(t, filepath.Join(cfg.Storage.AttachmentDir, "ticket.txt"), "attachment")
	assertFile(t, filepath.Join(cfg.Storage.TicketArchiveDir, "ticket.pdf"), "archive")
	assertFile(t, cfg.ConfigPath, "app:\n  env: test\n")

	restoredDB, err := database.Open(cfg.Storage.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	restoredSQLDB, err := restoredDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer restoredSQLDB.Close()
	var count int64
	if err := restoredDB.Model(&model.Asset{}).Where("asset_no = ?", "A-1").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("restored database count=%d err=%v", count, err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertFile(t *testing.T, path, expected string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != expected {
		t.Fatalf("%s=%q expected %q", path, raw, expected)
	}
}
