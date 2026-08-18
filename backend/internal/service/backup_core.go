package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"asset-registration-management-system/backend/internal/config"

	"gorm.io/gorm"
)

const (
	backupExtension = ".abk"
	backupHeader    = "ARMSBK1\n"
	backupChunkSize = 1024 * 1024
)

type BackupContent struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type BackupManifest struct {
	Version   int             `json:"version"`
	CreatedAt time.Time       `json:"createdAt"`
	Contents  []BackupContent `json:"contents"`
}

type BackupInfo struct {
	Name          string          `json:"name"`
	Size          int64           `json:"size"`
	ModTime       time.Time       `json:"modTime"`
	SHA256        string          `json:"sha256,omitempty"`
	Encrypted     bool            `json:"encrypted"`
	OffsiteCopied bool            `json:"offsiteCopied"`
	Contents      []BackupContent `json:"contents,omitempty"`
	VerifiedAt    *time.Time      `json:"verifiedAt,omitempty"`
	VerifyError   string          `json:"verifyError,omitempty"`
}

type BackupService struct {
	DB            *gorm.DB
	DBPath        string
	AttachmentDir string
	ArchiveDir    string
	ConfigPath    string
	BackupDir     string
	OffsiteDir    string
	KeepDays      int
	EncryptionKey string
}

func NewBackupService(db *gorm.DB, dbPath, backupDir string, keepDays int) *BackupService {
	return &BackupService{DB: db, DBPath: dbPath, BackupDir: backupDir, KeepDays: keepDays}
}

func NewFullBackupService(db *gorm.DB, cfg config.Config) *BackupService {
	return &BackupService{
		DB:            db,
		DBPath:        cfg.Storage.DatabasePath,
		AttachmentDir: cfg.Storage.AttachmentDir,
		ArchiveDir:    cfg.Storage.TicketArchiveDir,
		ConfigPath:    cfg.ConfigPath,
		BackupDir:     cfg.Storage.BackupDir,
		OffsiteDir:    cfg.Storage.BackupOffsiteDir,
		KeepDays:      cfg.Storage.BackupKeepDays,
		EncryptionKey: cfg.Security.ConfigEncryptionKey,
	}
}

func (s *BackupService) Create() (BackupInfo, error) {
	if s.DB == nil {
		return BackupInfo{}, errors.New("database is not open")
	}
	if err := os.MkdirAll(s.BackupDir, 0o700); err != nil {
		return BackupInfo{}, err
	}
	name := "backup-" + time.Now().Format("20060102-150405") + backupExtension
	target := filepath.Join(s.BackupDir, name)
	workspace, err := os.MkdirTemp(s.BackupDir, ".building-")
	if err != nil {
		return BackupInfo{}, err
	}
	defer os.RemoveAll(workspace)

	snapshot := filepath.Join(workspace, "assets.db")
	if err := s.DB.Exec("VACUUM INTO ?", snapshot).Error; err != nil {
		return BackupInfo{}, err
	}
	plainArchive := filepath.Join(workspace, "backup.zip")
	manifest, err := s.createArchive(plainArchive, snapshot)
	if err != nil {
		return BackupInfo{}, err
	}
	if err := encryptBackupFile(plainArchive, target, s.EncryptionKey); err != nil {
		return BackupInfo{}, err
	}
	info, err := s.buildInfo(name, manifest)
	if err != nil {
		return BackupInfo{}, err
	}
	if s.OffsiteDir != "" {
		if err := os.MkdirAll(s.OffsiteDir, 0o700); err != nil {
			return BackupInfo{}, fmt.Errorf("create offsite directory: %w", err)
		}
		if err := copyFile(target, filepath.Join(s.OffsiteDir, name), 0o600); err != nil {
			return BackupInfo{}, fmt.Errorf("copy offsite backup: %w", err)
		}
		info.OffsiteCopied = true
	}
	if err := s.writeMetadata(info); err != nil {
		return BackupInfo{}, err
	}
	if info.OffsiteCopied {
		_ = copyFile(s.metadataPath(name), filepath.Join(s.OffsiteDir, name+".json"), 0o600)
	}
	s.prune()
	return info, nil
}
