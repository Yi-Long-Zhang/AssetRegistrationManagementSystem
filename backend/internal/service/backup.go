package service

import (
	"archive/zip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

func (s *BackupService) createArchive(path, snapshot string) (BackupManifest, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return BackupManifest{}, err
	}
	writer := zip.NewWriter(file)
	manifest := BackupManifest{Version: 1, CreatedAt: time.Now().UTC(), Contents: []BackupContent{}}
	add := func(source, archivePath string) error {
		content, err := addFileToZip(writer, source, archivePath)
		if err == nil {
			manifest.Contents = append(manifest.Contents, content)
		}
		return err
	}
	if err := add(snapshot, "database/assets.db"); err != nil {
		writer.Close()
		file.Close()
		return BackupManifest{}, err
	}
	for _, item := range []struct {
		source string
		prefix string
	}{
		{s.AttachmentDir, "attachments"},
		{s.ArchiveDir, "ticket-archives"},
	} {
		if err := addDirectoryToZip(writer, item.source, item.prefix, add); err != nil {
			writer.Close()
			file.Close()
			return BackupManifest{}, err
		}
	}
	if s.ConfigPath != "" {
		if _, err := os.Stat(s.ConfigPath); err == nil {
			if err := add(s.ConfigPath, "config/config.yaml"); err != nil {
				writer.Close()
				file.Close()
				return BackupManifest{}, err
			}
		}
	}
	rawManifest, err := json.MarshalIndent(manifest, "", "  ")
	if err == nil {
		var entry io.Writer
		entry, err = writer.Create("manifest.json")
		if err == nil {
			_, err = entry.Write(rawManifest)
		}
	}
	closeErr := writer.Close()
	fileErr := file.Close()
	if err != nil {
		return BackupManifest{}, err
	}
	if closeErr != nil {
		return BackupManifest{}, closeErr
	}
	if fileErr != nil {
		return BackupManifest{}, fileErr
	}
	return manifest, nil
}

func addDirectoryToZip(writer *zip.Writer, source, prefix string, add func(string, string) error) error {
	if source == "" {
		return nil
	}
	if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		return add(path, filepath.ToSlash(filepath.Join(prefix, relative)))
	})
}

func addFileToZip(writer *zip.Writer, source, archivePath string) (BackupContent, error) {
	file, err := os.Open(source)
	if err != nil {
		return BackupContent{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return BackupContent{}, err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return BackupContent{}, err
	}
	header.Name = archivePath
	header.Method = zip.Deflate
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return BackupContent{}, err
	}
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(entry, hash), file)
	if err != nil {
		return BackupContent{}, err
	}
	return BackupContent{Path: archivePath, Size: size, SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}

func encryptBackupFile(source, target, keyText string) error {
	sourceFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	targetFile, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	success := false
	defer func() {
		targetFile.Close()
		if !success {
			_ = os.Remove(target)
		}
	}()
	block, err := aes.NewCipher(encryptionKey(keyText))
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	if _, err := targetFile.Write([]byte(backupHeader)); err != nil {
		return err
	}
	buffer := make([]byte, backupChunkSize)
	for {
		count, readErr := sourceFile.Read(buffer)
		if count > 0 {
			nonce := make([]byte, gcm.NonceSize())
			if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
				return err
			}
			sealed := gcm.Seal(nil, nonce, buffer[:count], nil)
			if err := binary.Write(targetFile, binary.BigEndian, uint32(len(sealed))); err != nil {
				return err
			}
			if _, err := targetFile.Write(nonce); err != nil {
				return err
			}
			if _, err := targetFile.Write(sealed); err != nil {
				return err
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if err := targetFile.Sync(); err != nil {
		return err
	}
	success = true
	return nil
}

func decryptBackupFile(source, target, keyText string) error {
	sourceFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	header := make([]byte, len(backupHeader))
	if _, err := io.ReadFull(sourceFile, header); err != nil || string(header) != backupHeader {
		return errors.New("invalid encrypted backup header")
	}
	block, err := aes.NewCipher(encryptionKey(keyText))
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	targetFile, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	success := false
	defer func() {
		targetFile.Close()
		if !success {
			_ = os.Remove(target)
		}
	}()
	for {
		var size uint32
		if err := binary.Read(sourceFile, binary.BigEndian, &size); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if size == 0 || size > backupChunkSize+1024 {
			return errors.New("invalid encrypted backup chunk")
		}
		nonce := make([]byte, gcm.NonceSize())
		if _, err := io.ReadFull(sourceFile, nonce); err != nil {
			return err
		}
		sealed := make([]byte, size)
		if _, err := io.ReadFull(sourceFile, sealed); err != nil {
			return err
		}
		plain, err := gcm.Open(nil, nonce, sealed, nil)
		if err != nil {
			return errors.New("backup decryption or integrity verification failed")
		}
		if _, err := targetFile.Write(plain); err != nil {
			return err
		}
	}
	if err := targetFile.Sync(); err != nil {
		return err
	}
	success = true
	return nil
}

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

type BackupScheduler struct {
	service *BackupService
	stop    chan struct{}
	tasks   *TaskManager
}

func NewBackupScheduler(service *BackupService) *BackupScheduler {
	return &BackupScheduler{service: service, stop: make(chan struct{})}
}

func (s *BackupScheduler) WithTaskManager(tasks *TaskManager) *BackupScheduler {
	s.tasks = tasks
	return s
}

func (s *BackupScheduler) Start() {
	if s.tasks != nil {
		s.tasks.Register("complete_backup", s.runTask)
		s.tasks.ResumeKind(context.Background(), "complete_backup")
	}
	go s.loop()
}

func (s *BackupScheduler) Stop() {
	close(s.stop)
}

func (s *BackupScheduler) loop() {
	s.backupOnce()
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.backupOnce()
		case <-s.stop:
			return
		}
	}
}

func (s *BackupScheduler) backupOnce() {
	if s.tasks != nil {
		today := time.Now().Format("2006-01-02")
		if _, err := s.tasks.Run(context.Background(), "complete_backup", "schedule",
			"backup:"+today, map[string]interface{}{"date": today}); err != nil {
			log.Printf("backup scheduler: %v", err)
		}
		return
	}
	_, err := s.createAndVerify()
	if err != nil {
		log.Printf("backup scheduler: %v", err)
	}
}

func (s *BackupScheduler) runTask(_ context.Context, _ json.RawMessage) (interface{}, error) {
	return s.createAndVerify()
}

func (s *BackupScheduler) createAndVerify() (interface{}, error) {
	info, err := s.service.Create()
	if err != nil {
		return nil, err
	}
	verified, err := s.service.Verify(info.Name)
	if err != nil {
		return nil, fmt.Errorf("backup restore drill %s: %w", info.Name, err)
	}
	return map[string]interface{}{"backup": verified.Name, "sha256": verified.SHA256}, nil
}
