package service

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *BackupService) Verify(name string) (BackupInfo, error) {
	info, err := s.Get(name)
	if err != nil {
		return info, err
	}
	if strings.HasSuffix(name, ".db") {
		err = verifySQLiteFile(filepath.Join(s.BackupDir, name))
	} else {
		err = s.verifyFullBackup(filepath.Join(s.BackupDir, name), info)
	}
	now := time.Now().UTC()
	info.VerifiedAt = &now
	if err != nil {
		info.VerifyError = err.Error()
	} else {
		info.VerifyError = ""
	}
	_ = s.writeMetadata(info)
	return info, err
}

func (s *BackupService) verifyFullBackup(path string, info BackupInfo) error {
	checksum, _, err := hashFile(path)
	if err != nil {
		return err
	}
	if info.SHA256 != "" && checksum != info.SHA256 {
		return errors.New("backup SHA-256 mismatch")
	}
	workspace, err := os.MkdirTemp(s.BackupDir, ".verify-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workspace)
	plain := filepath.Join(workspace, "backup.zip")
	if err := decryptBackupFile(path, plain, s.EncryptionKey); err != nil {
		return err
	}
	reader, err := zip.OpenReader(plain)
	if err != nil {
		return err
	}
	defer reader.Close()
	manifest, err := readBackupManifest(reader.File)
	if err != nil {
		return err
	}
	expected := make(map[string]BackupContent, len(manifest.Contents))
	for _, item := range manifest.Contents {
		expected[item.Path] = item
	}
	var databaseChecked bool
	for _, file := range reader.File {
		item, ok := expected[file.Name]
		if !ok {
			continue
		}
		hash, size, err := hashZipFile(file)
		if err != nil {
			return err
		}
		if hash != item.SHA256 || size != item.Size {
			return fmt.Errorf("backup content mismatch: %s", file.Name)
		}
		if file.Name == "database/assets.db" {
			databaseChecked = true
			stream, err := file.Open()
			if err != nil {
				return err
			}
			header := make([]byte, 16)
			_, err = io.ReadFull(stream, header)
			stream.Close()
			if err != nil || string(header) != "SQLite format 3\x00" {
				return errors.New("backup database is invalid")
			}
		}
		delete(expected, file.Name)
	}
	if !databaseChecked || len(expected) != 0 {
		return errors.New("backup manifest is incomplete")
	}
	return nil
}

func readBackupManifest(files []*zip.File) (BackupManifest, error) {
	for _, file := range files {
		if file.Name != "manifest.json" {
			continue
		}
		stream, err := file.Open()
		if err != nil {
			return BackupManifest{}, err
		}
		defer stream.Close()
		var manifest BackupManifest
		if err := json.NewDecoder(stream).Decode(&manifest); err != nil {
			return BackupManifest{}, err
		}
		return manifest, nil
	}
	return BackupManifest{}, errors.New("backup manifest is missing")
}

func hashZipFile(file *zip.File) (string, int64, error) {
	stream, err := file.Open()
	if err != nil {
		return "", 0, err
	}
	defer stream.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, stream)
	return hex.EncodeToString(hash.Sum(nil)), size, err
}

func (s *BackupService) Restore(name string) error {
	if filepath.Base(name) != name {
		return errors.New("非法备份名")
	}
	source := filepath.Join(s.BackupDir, name)
	if strings.HasSuffix(name, ".db") {
		if err := verifySQLiteFile(source); err != nil {
			return err
		}
		return copyFile(source, s.restorePendingPath(), 0o600)
	}
	if !strings.HasSuffix(name, backupExtension) {
		return errors.New("非法备份名")
	}
	if _, err := s.Verify(name); err != nil {
		return err
	}
	pending := s.restoreSetPath()
	if err := os.RemoveAll(pending); err != nil {
		return err
	}
	if err := os.MkdirAll(pending, 0o700); err != nil {
		return err
	}
	plain := filepath.Join(pending, ".restore.zip")
	if err := decryptBackupFile(source, plain, s.EncryptionKey); err != nil {
		return err
	}
	if err := extractBackup(plain, pending); err != nil {
		return err
	}
	return os.Remove(plain)
}

func extractBackup(archivePath, target string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	root := filepath.Clean(target) + string(os.PathSeparator)
	for _, file := range reader.File {
		if file.Name == "manifest.json" {
			continue
		}
		destination := filepath.Join(target, filepath.FromSlash(file.Name))
		if !strings.HasPrefix(filepath.Clean(destination), root) {
			return errors.New("backup contains an invalid path")
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			return err
		}
		source, err := file.Open()
		if err != nil {
			return err
		}
		output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			source.Close()
			return err
		}
		_, copyErr := io.Copy(output, source)
		closeErr := output.Close()
		source.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func (s *BackupService) ApplyPendingRestore() (bool, error) {
	if pending := s.restorePendingPath(); fileExists(pending) {
		if err := replacePath(pending, s.DBPath); err != nil {
			return false, err
		}
		return true, nil
	}
	root := s.restoreSetPath()
	if !fileExists(root) {
		return false, nil
	}
	type replacement struct {
		staged string
		target string
	}
	replacements := []replacement{
		{filepath.Join(root, "database", "assets.db"), s.DBPath},
		{filepath.Join(root, "attachments"), s.AttachmentDir},
		{filepath.Join(root, "ticket-archives"), s.ArchiveDir},
		{filepath.Join(root, "config", "config.yaml"), s.ConfigPath},
	}
	for _, item := range replacements {
		if item.target == "" || !fileExists(item.staged) {
			continue
		}
		if err := replacePath(item.staged, item.target); err != nil {
			return false, err
		}
	}
	if err := os.RemoveAll(root); err != nil {
		return false, err
	}
	return true, nil
}
