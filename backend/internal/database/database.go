package database

import (
	"errors"
	"os"
	"path/filepath"

	"asset-registration-management-system/backend/internal/model"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func Open(path string) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return gorm.Open(sqlite.Open(path), &gorm.Config{})
}

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{},
		&model.Asset{},
		&model.Ticket{},
		&model.TicketTypeApprover{},
		&model.TicketComment{},
		&model.TicketAttachment{},
		&model.TicketRecord{},
		&model.AuditLog{},
	)
}

func SeedAdmin(db *gorm.DB, username, password string) error {
	var existing model.User
	err := db.Where("username = ?", username).First(&existing).Error
	if err == nil {
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
		Username:     username,
		Name:         "系统管理员",
		Role:         model.RoleAdmin,
		Status:       "active",
		PasswordHash: string(hash),
	}).Error
}
