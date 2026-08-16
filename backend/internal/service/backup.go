package service

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

// BackupInfo 单个备份文件的元信息。
type BackupInfo struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
}

// BackupService 数据库备份/恢复（SQLite VACUUM INTO 一致性快照）。
type BackupService struct {
	DB        *gorm.DB
	DBPath    string
	BackupDir string
	KeepDays  int
}

func NewBackupService(db *gorm.DB, dbPath, backupDir string, keepDays int) *BackupService {
	return &BackupService{DB: db, DBPath: dbPath, BackupDir: backupDir, KeepDays: keepDays}
}

// Create 生成一份一致性备份（VACUUM INTO），并清理超过保留天数的旧备份。
func (s *BackupService) Create() (BackupInfo, error) {
	if err := os.MkdirAll(s.BackupDir, 0o755); err != nil {
		return BackupInfo{}, err
	}
	name := "backup-" + time.Now().Format("20060102-150405") + ".db"
	path := filepath.Join(s.BackupDir, name)
	if err := s.DB.Exec("VACUUM INTO ?", path).Error; err != nil {
		return BackupInfo{}, err
	}
	s.prune()
	info, err := os.Stat(path)
	if err != nil {
		return BackupInfo{}, err
	}
	return BackupInfo{Name: name, Size: info.Size(), ModTime: info.ModTime()}, nil
}

// List 列出所有备份（按时间倒序）。
func (s *BackupService) List() ([]BackupInfo, error) {
	entries, err := os.ReadDir(s.BackupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupInfo{}, nil
		}
		return nil, err
	}
	list := make([]BackupInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".db") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		list = append(list, BackupInfo{Name: entry.Name(), Size: info.Size(), ModTime: info.ModTime()})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ModTime.After(list[j].ModTime) })
	return list, nil
}

// Delete 删除指定备份（防路径穿越）。
func (s *BackupService) Delete(name string) error {
	if filepath.Base(name) != name || !strings.HasSuffix(name, ".db") {
		return fmt.Errorf("非法备份名")
	}
	return os.Remove(filepath.Join(s.BackupDir, name))
}

// Restore 校验备份有效性并标记为待恢复（复制到 <DBPath>.restore），后端重启后生效。
func (s *BackupService) Restore(name string) error {
	if filepath.Base(name) != name || !strings.HasSuffix(name, ".db") {
		return fmt.Errorf("非法备份名")
	}
	src := filepath.Join(s.BackupDir, name)
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("备份不存在")
	}
	if err := verifySQLiteFile(src); err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(s.restorePendingPath(), data, 0o644)
}

func (s *BackupService) restorePendingPath() string {
	return s.DBPath + ".restore"
}

// ApplyPendingRestore 启动时若存在待恢复标记，用备份替换当前库文件（返回是否发生了替换）。
func (s *BackupService) ApplyPendingRestore() (bool, error) {
	pending := s.restorePendingPath()
	if _, err := os.Stat(pending); err != nil {
		return false, nil // 无待恢复标记
	}
	if err := os.Rename(pending, s.DBPath); err != nil {
		return false, err
	}
	return true, nil
}

// prune 清理超过保留天数的旧备份。
func (s *BackupService) prune() {
	if s.KeepDays <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -s.KeepDays)
	entries, _ := os.ReadDir(s.BackupDir)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".db") {
			continue
		}
		info, err := entry.Info()
		if err == nil && info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(s.BackupDir, entry.Name()))
		}
	}
}

// verifySQLiteFile 校验 SQLite 文件头（"SQLite format 3\0"）。
func verifySQLiteFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	header := make([]byte, 16)
	if _, err := io.ReadFull(file, header); err != nil {
		return fmt.Errorf("备份文件无效")
	}
	if string(header) != "SQLite format 3\x00" {
		return fmt.Errorf("备份文件不是有效的 SQLite 数据库")
	}
	return nil
}

// BackupScheduler 每日自动备份（进程内 ticker）。
type BackupScheduler struct {
	service *BackupService
	stop    chan struct{}
}

func NewBackupScheduler(service *BackupService) *BackupScheduler {
	return &BackupScheduler{service: service, stop: make(chan struct{})}
}

func (s *BackupScheduler) Start() {
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
	if _, err := s.service.Create(); err != nil {
		log.Printf("backup scheduler: %v", err)
	}
}
