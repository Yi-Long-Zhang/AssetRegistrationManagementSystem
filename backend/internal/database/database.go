package database

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"asset-registration-management-system/backend/internal/model"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type SchemaMigration struct {
	Version   int       `gorm:"primaryKey"`
	Name      string    `gorm:"size:128;not null"`
	AppliedAt time.Time `gorm:"not null"`
}

type MigrationStatus struct {
	CurrentVersion int      `json:"currentVersion"`
	LatestVersion  int      `json:"latestVersion"`
	Pending        []string `json:"pending"`
}

type migration struct {
	version int
	name    string
	up      func(*gorm.DB) error
}

func Open(path string) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return gorm.Open(sqlite.Open(path), &gorm.Config{})
}

var migrations = []migration{
	{
		version: 1,
		name:    "baseline_schema",
		up: func(db *gorm.DB) error {
			return db.AutoMigrate(baselineModels()...)
		},
	},
	{
		version: 2,
		name:    "production_hardening",
		up: func(db *gorm.DB) error {
			return db.AutoMigrate(
				&model.User{},
				&model.AuthSession{},
				&model.BackgroundTask{},
			)
		},
	},
}

func baselineModels() []interface{} {
	return []interface{}{
		&model.User{},
		&model.ADConfig{},
		&model.MailConfig{},
		&model.IMConfig{},
		&model.IMBinding{},
		&model.IMCallbackConfig{},
		&model.Asset{},
		&model.Credential{},
		&model.SoftwareLicense{},
		&model.LicenseAttachment{},
		&model.Ticket{},
		&model.TicketTypeApprover{},
		&model.TicketWorkflow{},
		&model.TicketWorkflowNode{},
		&model.TicketWorkflowNodeApprover{},
		&model.TicketWorkflowStep{},
		&model.TicketWorkflowStepApprover{},
		&model.TicketComment{},
		&model.TicketAttachment{},
		&model.TicketRecord{},
		&model.TicketAsset{},
		&model.StocktakeTask{},
		&model.StocktakeItem{},
		&model.DatacenterRoom{},
		&model.Rack{},
		&model.AuditLog{},
		&model.DiscoveryRule{},
		&model.InspectionRule{},
		&model.DiscoveryRun{},
		&model.DiscoveredHost{},
		&model.AssetSnapshot{},
		&model.IPSegment{},
	}
}

// Migrate applies each pending schema version in its own transaction.
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&SchemaMigration{}); err != nil {
		return fmt.Errorf("create schema migrations table: %w", err)
	}
	for _, item := range migrations {
		var count int64
		if err := db.Model(&SchemaMigration{}).Where("version = ?", item.version).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			continue
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := item.up(tx); err != nil {
				return err
			}
			return tx.Create(&SchemaMigration{
				Version:   item.version,
				Name:      item.name,
				AppliedAt: time.Now().UTC(),
			}).Error
		}); err != nil {
			return fmt.Errorf("migration %d %s: %w", item.version, item.name, err)
		}
	}
	return nil
}

func MigrationState(db *gorm.DB) (MigrationStatus, error) {
	status := MigrationStatus{LatestVersion: migrations[len(migrations)-1].version, Pending: []string{}}
	if !db.Migrator().HasTable(&SchemaMigration{}) {
		for _, item := range migrations {
			status.Pending = append(status.Pending, item.name)
		}
		return status, nil
	}
	if err := db.Model(&SchemaMigration{}).Select("COALESCE(MAX(version), 0)").Scan(&status.CurrentVersion).Error; err != nil {
		return status, err
	}
	for _, item := range migrations {
		if item.version > status.CurrentVersion {
			status.Pending = append(status.Pending, item.name)
		}
	}
	return status, nil
}

func SeedAdmin(db *gorm.DB, username, password string) error {
	var existing model.User
	err := db.Where("username = ?", username).First(&existing).Error
	if err == nil {
		if existing.SessionVersion == 0 {
			return db.Model(&existing).Update("session_version", 1).Error
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return db.Create(&model.User{
		Username:           username,
		Name:               "系统管理员",
		Role:               model.RoleAdmin,
		Status:             "active",
		PasswordHash:       string(hash),
		MustChangePassword: true,
		SessionVersion:     1,
	}).Error
}
