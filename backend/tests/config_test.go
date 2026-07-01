package tests

import (
	"os"
	"path/filepath"
	"testing"

	"asset-registration-management-system/backend/internal/config"
)

func TestLoadUsesDevelopmentDefaultsWhenNoConfigFileExists(t *testing.T) {
	t.Setenv("CONFIG_FILE", "")
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.Env != config.EnvDevelopment {
		t.Fatalf("expected development env, got %s", cfg.App.Env)
	}
	if cfg.HTTP.Addr == "" || cfg.Storage.DatabasePath == "" || cfg.Security.JWTSecret == "" {
		t.Fatalf("expected default config values, got %+v", cfg)
	}
}

func TestLoadReturnsErrorForExplicitMissingConfigFile(t *testing.T) {
	t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "missing.yaml"))
	if _, err := config.Load(); err == nil {
		t.Fatal("expected explicit missing CONFIG_FILE to fail")
	}
}

func TestLoadFileReadsYAMLConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	raw := []byte(`
app:
  env: production
http:
  addr: 127.0.0.1:9000
storage:
  database_path: /data/assets.db
  attachment_dir: /data/attachments
  ticket_archive_dir: /data/archives
  ticket_template_path: /templates/ticket.docx
  libreoffice_bin: /usr/bin/soffice
security:
  jwt_secret: strong-jwt-secret
  config_encryption_key: strong-config-key
auth:
  mode: local
swagger:
  enabled: true
admin:
  username: root
  password: strong-admin-password
cors:
  allowed_origins: https://example.com
`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.Env != config.EnvProduction ||
		cfg.HTTP.Addr != "127.0.0.1:9000" ||
		cfg.Storage.DatabasePath != "/data/assets.db" ||
		cfg.Storage.AttachmentDir != "/data/attachments" ||
		cfg.Storage.TicketArchiveDir != "/data/archives" ||
		cfg.Storage.TicketTemplatePath != "/templates/ticket.docx" ||
		cfg.Storage.LibreOfficeBin != "/usr/bin/soffice" ||
		cfg.Security.JWTSecret != "strong-jwt-secret" ||
		cfg.Security.ConfigEncryptionKey != "strong-config-key" ||
		cfg.Auth.Mode != "local" ||
		!cfg.Swagger.Enabled ||
		cfg.Admin.Username != "root" ||
		cfg.Admin.Password != "strong-admin-password" ||
		cfg.CORS.AllowedOrigins != "https://example.com" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestValidateRejectsDefaultSecretsInProduction(t *testing.T) {
	cfg := config.Default()
	cfg.App.Env = config.EnvProduction
	cfg.Security.ConfigEncryptionKey = "strong-config-key"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected default jwt secret to fail in production")
	}

	cfg = config.Default()
	cfg.App.Env = config.EnvProduction
	cfg.Security.JWTSecret = "strong-jwt-secret"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected default config encryption key to fail in production")
	}
}

func TestValidateAllowsDevelopmentDefaults(t *testing.T) {
	cfg := config.Default()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("development defaults should be allowed: %v", err)
	}
}
