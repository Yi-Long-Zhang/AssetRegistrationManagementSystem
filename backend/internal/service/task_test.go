package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"asset-registration-management-system/backend/internal/database"
	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"
)

func TestTaskManagerIdempotencyRetryAndRecovery(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}

	manager := service.NewTaskManager(db)
	var successRuns atomic.Int32
	manager.Register("success", func(context.Context, json.RawMessage) (interface{}, error) {
		successRuns.Add(1)
		return map[string]bool{"ok": true}, nil
	})
	first, err := manager.Run(context.Background(), "success", "test", "same-key", map[string]int{"id": 1})
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.Run(context.Background(), "success", "test", "same-key", map[string]int{"id": 1})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || successRuns.Load() != 1 {
		t.Fatalf("idempotency failed first=%d second=%d runs=%d", first.ID, second.ID, successRuns.Load())
	}

	var attempts atomic.Int32
	manager.Register("retry", func(context.Context, json.RawMessage) (interface{}, error) {
		if attempts.Add(1) == 1 {
			return nil, errors.New("temporary failure")
		}
		return "recovered", nil
	})
	failed, err := manager.Run(context.Background(), "retry", "test", "retry-key", nil)
	if err == nil || failed.Status != model.BackgroundTaskFailed {
		t.Fatalf("expected failed task, task=%+v err=%v", failed, err)
	}
	retried, err := manager.Retry(context.Background(), failed.ID)
	if err != nil || retried.Status != model.BackgroundTaskSucceeded {
		t.Fatalf("retry task=%+v err=%v", retried, err)
	}

	interrupted := model.BackgroundTask{
		Kind: "success", Source: "test", UniqueKey: "interrupted",
		Status: model.BackgroundTaskRunning, ScheduledAt: time.Now(), MaxAttempts: 3,
	}
	if err := db.Create(&interrupted).Error; err != nil {
		t.Fatal(err)
	}
	if err := manager.RecoverInterrupted(); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&interrupted, interrupted.ID).Error; err != nil {
		t.Fatal(err)
	}
	if interrupted.Status != model.BackgroundTaskQueued {
		t.Fatalf("interrupted status=%s", interrupted.Status)
	}
}
