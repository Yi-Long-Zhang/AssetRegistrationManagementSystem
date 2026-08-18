package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

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
