package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func replacePath(source, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	previous := target + ".pre-restore"
	_ = os.RemoveAll(previous)
	if fileExists(target) {
		if err := os.Rename(target, previous); err != nil {
			return err
		}
	}
	if err := os.Rename(source, target); err != nil {
		if fileExists(previous) {
			_ = os.Rename(previous, target)
		}
		return err
	}
	return os.RemoveAll(previous)
}

func (s *BackupService) List() ([]BackupInfo, error) {
	entries, err := os.ReadDir(s.BackupDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []BackupInfo{}, nil
		}
		return nil, err
	}
	list := make([]BackupInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), backupExtension) && !strings.HasSuffix(entry.Name(), ".db")) {
			continue
		}
		item, err := s.Get(entry.Name())
		if err == nil {
			list = append(list, item)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ModTime.After(list[j].ModTime) })
	return list, nil
}

func (s *BackupService) Get(name string) (BackupInfo, error) {
	if filepath.Base(name) != name || (!strings.HasSuffix(name, backupExtension) && !strings.HasSuffix(name, ".db")) {
		return BackupInfo{}, errors.New("非法备份名")
	}
	if strings.HasSuffix(name, backupExtension) {
		var info BackupInfo
		if raw, err := os.ReadFile(s.metadataPath(name)); err == nil {
			if json.Unmarshal(raw, &info) == nil {
				return info, nil
			}
		}
	}
	path := filepath.Join(s.BackupDir, name)
	stat, err := os.Stat(path)
	if err != nil {
		return BackupInfo{}, err
	}
	checksum, _, err := hashFile(path)
	if err != nil {
		return BackupInfo{}, err
	}
	return BackupInfo{
		Name: name, Size: stat.Size(), ModTime: stat.ModTime(),
		SHA256: checksum, Encrypted: strings.HasSuffix(name, backupExtension),
	}, nil
}

func (s *BackupService) Delete(name string) error {
	if _, err := s.Get(name); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(s.BackupDir, name)); err != nil {
		return err
	}
	_ = os.Remove(s.metadataPath(name))
	if s.OffsiteDir != "" {
		_ = os.Remove(filepath.Join(s.OffsiteDir, name))
		_ = os.Remove(filepath.Join(s.OffsiteDir, name+".json"))
	}
	return nil
}

func (s *BackupService) Path(name string) (string, error) {
	if _, err := s.Get(name); err != nil {
		return "", err
	}
	return filepath.Join(s.BackupDir, name), nil
}

func (s *BackupService) buildInfo(name string, manifest BackupManifest) (BackupInfo, error) {
	path := filepath.Join(s.BackupDir, name)
	checksum, size, err := hashFile(path)
	if err != nil {
		return BackupInfo{}, err
	}
	stat, err := os.Stat(path)
	if err != nil {
		return BackupInfo{}, err
	}
	return BackupInfo{
		Name: name, Size: size, ModTime: stat.ModTime(), SHA256: checksum,
		Encrypted: true, Contents: manifest.Contents,
	}, nil
}

func (s *BackupService) writeMetadata(info BackupInfo) error {
	if !strings.HasSuffix(info.Name, backupExtension) {
		return nil
	}
	raw, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.metadataPath(info.Name), raw, 0o600)
}

func (s *BackupService) metadataPath(name string) string {
	return filepath.Join(s.BackupDir, name+".json")
}

func (s *BackupService) restorePendingPath() string {
	return s.DBPath + ".restore"
}

func (s *BackupService) restoreSetPath() string {
	return s.DBPath + ".restore-set"
}

func (s *BackupService) prune() {
	if s.KeepDays <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -s.KeepDays)
	entries, _ := os.ReadDir(s.BackupDir)
	for _, entry := range entries {
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), backupExtension) && !strings.HasSuffix(entry.Name(), ".db")) {
			continue
		}
		info, err := entry.Info()
		if err == nil && info.ModTime().Before(cutoff) {
			_ = s.Delete(entry.Name())
		}
	}
}

func verifySQLiteFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	header := make([]byte, 16)
	if _, err := io.ReadFull(file, header); err != nil {
		return errors.New("备份文件无效")
	}
	if string(header) != "SQLite format 3\x00" {
		return errors.New("备份文件不是有效的 SQLite 数据库")
	}
	return nil
}

func hashFile(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	return hex.EncodeToString(hash.Sum(nil)), size, err
}

func copyFile(source, target string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
