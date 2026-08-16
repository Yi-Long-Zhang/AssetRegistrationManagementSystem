package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvProduction  = "production"

	DefaultJWTSecret = "change-me-in-production"
	DefaultConfigKey = "change-me-config-key"
)

type Config struct {
	App       AppConfig       `yaml:"app"`
	HTTP      HTTPConfig      `yaml:"http"`
	Storage   StorageConfig   `yaml:"storage"`
	Security  SecurityConfig  `yaml:"security"`
	Auth      AuthConfig      `yaml:"auth"`
	Swagger   SwaggerConfig   `yaml:"swagger"`
	Admin     AdminConfig     `yaml:"admin"`
	CORS      CORSConfig      `yaml:"cors"`
	Discovery DiscoveryConfig `yaml:"discovery"`
	TokenTTL  time.Duration   `yaml:"-"`
}

type AppConfig struct {
	Env string `yaml:"env"`
}

type HTTPConfig struct {
	Addr string `yaml:"addr"`
}

type StorageConfig struct {
	DatabasePath       string `yaml:"database_path"`
	AttachmentDir      string `yaml:"attachment_dir"`
	TicketArchiveDir   string `yaml:"ticket_archive_dir"`
	TicketTemplatePath string `yaml:"ticket_template_path"`
	LibreOfficeBin     string `yaml:"libreoffice_bin"`
	BackupDir          string `yaml:"backup_dir"`
	BackupKeepDays     int    `yaml:"backup_keep_days"`
}

type SecurityConfig struct {
	JWTSecret           string `yaml:"jwt_secret"`
	ConfigEncryptionKey string `yaml:"config_encryption_key"`
}

type AuthConfig struct {
	Mode string `yaml:"mode"`
}

type SwaggerConfig struct {
	Enabled bool `yaml:"enabled"`
}

type AdminConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type CORSConfig struct {
	AllowedOrigins string `yaml:"allowed_origins"`
}

// DiscoveryConfig 资产自动发现（nmap）配置
type DiscoveryConfig struct {
	NmapBin           string `yaml:"nmap_bin"`            // 空=自动探测 tools/nmap/<platform>/ 与 PATH
	ScanTimeoutSec    int    `yaml:"scan_timeout_sec"`    // 单次扫描超时秒数
	DefaultPorts      string `yaml:"default_ports"`       // 规则未指定端口时的默认端口列表
	ProbePorts        string `yaml:"probe_ports"`         // 两阶段扫描探活端口（快速端口）
	MaxParallelScans  int    `yaml:"max_parallel_scans"`  // 大网段分片并行扫描最大并发
	ScanChunkSize     int    `yaml:"scan_chunk_size"`     // 大网段单次扫描主机数上限
	MaxHosts          int    `yaml:"max_hosts"`           // 单次扫描最大主机数，防止误配大网段
	MinRate           int    `yaml:"min_rate"`            // nmap --min-rate（每秒发包下限），0=不限制
	MaxRate           int    `yaml:"max_rate"`            // nmap --max-rate（每秒发包上限），0=不限制
	AlertEmails       string `yaml:"alert_emails"`        // 变更告警收件人（逗号分隔），空=不发送
	OfflineAfterHours int    `yaml:"offline_after_hours"` // 资产连续未响应超过该小时数（且跨规则最近一轮缺席）才判离线
}

func Load() (Config, error) {
	return LoadFile(os.Getenv("CONFIG_FILE"))
}

func LoadFile(path string) (Config, error) {
	useDefaultPath := strings.TrimSpace(path) == ""
	if strings.TrimSpace(path) == "" {
		path = "config.yaml"
	}
	cfg := Default()
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && useDefaultPath {
			return cfg, nil
		}
		return Config{}, err
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, err
	}
	cfg.applyDefaults()
	return cfg, nil
}

func Default() Config {
	return defaultConfig()
}

func defaultConfig() Config {
	return Config{
		App: AppConfig{
			Env: EnvDevelopment,
		},
		HTTP: HTTPConfig{
			Addr: ":8080",
		},
		Storage: StorageConfig{
			DatabasePath:       "data/assets.db",
			AttachmentDir:      "data/attachments",
			TicketArchiveDir:   "data/ticket-archives",
			TicketTemplatePath: "../templates/ticket-it-change-template.docx",
			LibreOfficeBin:     "soffice",
			BackupDir:          "data/backups",
			BackupKeepDays:     30,
		},
		Security: SecurityConfig{
			JWTSecret:           DefaultJWTSecret,
			ConfigEncryptionKey: DefaultConfigKey,
		},
		Auth: AuthConfig{
			Mode: "mixed",
		},
		Swagger: SwaggerConfig{
			Enabled: false,
		},
		Admin: AdminConfig{
			Username: "admin",
			Password: "admin123456",
		},
		CORS: CORSConfig{
			AllowedOrigins: "*",
		},
		Discovery: DiscoveryConfig{
			NmapBin:           "",
			ScanTimeoutSec:    300,
			DefaultPorts:      "22,80,443,3389",
			ProbePorts:        "22,80,443,445,3389",
			MaxParallelScans:  4,
			ScanChunkSize:     128,
			MaxHosts:          1024,
			OfflineAfterHours: 24,
		},
		TokenTTL: 24 * time.Hour,
	}
}

func (c *Config) applyDefaults() {
	defaults := defaultConfig()
	if c.App.Env == "" {
		c.App.Env = defaults.App.Env
	}
	if c.HTTP.Addr == "" {
		c.HTTP.Addr = defaults.HTTP.Addr
	}
	if c.Storage.DatabasePath == "" {
		c.Storage.DatabasePath = defaults.Storage.DatabasePath
	}
	if c.Storage.AttachmentDir == "" {
		c.Storage.AttachmentDir = defaults.Storage.AttachmentDir
	}
	if c.Storage.TicketArchiveDir == "" {
		c.Storage.TicketArchiveDir = defaults.Storage.TicketArchiveDir
	}
	if c.Storage.TicketTemplatePath == "" {
		c.Storage.TicketTemplatePath = defaults.Storage.TicketTemplatePath
	}
	if c.Storage.LibreOfficeBin == "" {
		c.Storage.LibreOfficeBin = defaults.Storage.LibreOfficeBin
	}
	if c.Storage.BackupDir == "" {
		c.Storage.BackupDir = defaults.Storage.BackupDir
	}
	if c.Storage.BackupKeepDays == 0 {
		c.Storage.BackupKeepDays = defaults.Storage.BackupKeepDays
	}
	if c.Security.JWTSecret == "" {
		c.Security.JWTSecret = defaults.Security.JWTSecret
	}
	if c.Security.ConfigEncryptionKey == "" {
		c.Security.ConfigEncryptionKey = defaults.Security.ConfigEncryptionKey
	}
	if c.Auth.Mode == "" {
		c.Auth.Mode = defaults.Auth.Mode
	}
	if c.Admin.Username == "" {
		c.Admin.Username = defaults.Admin.Username
	}
	if c.Admin.Password == "" {
		c.Admin.Password = defaults.Admin.Password
	}
	if c.CORS.AllowedOrigins == "" {
		c.CORS.AllowedOrigins = defaults.CORS.AllowedOrigins
	}
	if c.Discovery.ScanTimeoutSec == 0 {
		c.Discovery.ScanTimeoutSec = defaults.Discovery.ScanTimeoutSec
	}
	if c.Discovery.DefaultPorts == "" {
		c.Discovery.DefaultPorts = defaults.Discovery.DefaultPorts
	}
	if c.Discovery.ProbePorts == "" {
		c.Discovery.ProbePorts = defaults.Discovery.ProbePorts
	}
	if c.Discovery.MaxParallelScans == 0 {
		c.Discovery.MaxParallelScans = defaults.Discovery.MaxParallelScans
	}
	if c.Discovery.ScanChunkSize == 0 {
		c.Discovery.ScanChunkSize = defaults.Discovery.ScanChunkSize
	}
	if c.Discovery.MaxHosts == 0 {
		c.Discovery.MaxHosts = defaults.Discovery.MaxHosts
	}
	if c.Discovery.OfflineAfterHours == 0 {
		c.Discovery.OfflineAfterHours = defaults.Discovery.OfflineAfterHours
	}
	if c.TokenTTL == 0 {
		c.TokenTTL = defaults.TokenTTL
	}
}

func (c Config) Validate() error {
	// 生产环境：默认密钥直接拒绝启动，防止误部署弱密钥。
	if strings.EqualFold(c.App.Env, EnvProduction) {
		if c.Security.JWTSecret == "" || c.Security.JWTSecret == DefaultJWTSecret {
			return fmt.Errorf("security.jwt_secret must be set to a non-default value in production")
		}
		if c.Security.ConfigEncryptionKey == "" || c.Security.ConfigEncryptionKey == DefaultConfigKey {
			return fmt.Errorf("security.config_encryption_key must be set to a non-default value in production")
		}
	} else {
		// 非生产环境使用默认密钥仅告警（开发便利），但仍提示潜在风险。
		if c.Security.JWTSecret == "" || c.Security.JWTSecret == DefaultJWTSecret {
			log.Printf("[security] WARNING: security.jwt_secret is using the default value; set a strong secret for any shared environment")
		}
		if c.Security.ConfigEncryptionKey == "" || c.Security.ConfigEncryptionKey == DefaultConfigKey {
			log.Printf("[security] WARNING: security.config_encryption_key is using the default value; set a strong key for any shared environment")
		}
	}
	return nil
}
