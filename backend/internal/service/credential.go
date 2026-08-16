package service

import (
	"errors"
	"time"

	"asset-registration-management-system/backend/internal/model"

	"gorm.io/gorm"
)

// CredentialService 凭据托管（敏感字段 AES-GCM 加密存储）。
type CredentialService struct {
	DB            *gorm.DB
	EncryptionKey string
}

func NewCredentialService(db *gorm.DB, encryptionKey string) *CredentialService {
	return &CredentialService{DB: db, EncryptionKey: encryptionKey}
}

// List 返回凭据列表（不含明文）。
func (s *CredentialService) List() ([]model.Credential, error) {
	var list []model.Credential
	err := s.DB.Preload("Asset").Order("id desc").Find(&list).Error
	return list, err
}

// Get 返回单个凭据（不含明文）。
func (s *CredentialService) Get(id uint) (model.Credential, error) {
	var cred model.Credential
	err := s.DB.Preload("Asset").First(&cred, id).Error
	return cred, err
}

// Create 加密 secret 后创建凭据。
func (s *CredentialService) Create(cred *model.Credential, plainSecret string) error {
	if plainSecret == "" {
		return errors.New("secret is required")
	}
	enc, err := EncryptString(plainSecret, s.EncryptionKey)
	if err != nil {
		return err
	}
	cred.EncryptedSecret = enc
	return s.DB.Create(cred).Error
}

// Update 更新凭据元信息；plainSecret 非空时重新加密存储。
func (s *CredentialService) Update(id uint, cred *model.Credential, plainSecret string) error {
	var existing model.Credential
	if err := s.DB.First(&existing, id).Error; err != nil {
		return err
	}
	existing.Name = cred.Name
	existing.Username = cred.Username
	existing.Type = cred.Type
	existing.AssetID = cred.AssetID
	existing.Remark = cred.Remark
	if plainSecret != "" {
		enc, err := EncryptString(plainSecret, s.EncryptionKey)
		if err != nil {
			return err
		}
		existing.EncryptedSecret = enc
	}
	return s.DB.Save(&existing).Error
}

// Delete 删除凭据（软删除）。
func (s *CredentialService) Delete(id uint) error {
	return s.DB.Delete(&model.Credential{}, id).Error
}

// Reveal 解密并返回明文 secret，同时更新最后访问时间。
func (s *CredentialService) Reveal(id uint) (model.Credential, string, error) {
	var cred model.Credential
	if err := s.DB.First(&cred, id).Error; err != nil {
		return cred, "", err
	}
	plain, err := DecryptString(cred.EncryptedSecret, s.EncryptionKey)
	if err != nil {
		return cred, "", err
	}
	now := time.Now()
	if err := s.DB.Model(&cred).Update("last_accessed_at", now).Error; err != nil {
		return cred, "", err
	}
	cred.LastAccessedAt = &now
	return cred, plain, nil
}
