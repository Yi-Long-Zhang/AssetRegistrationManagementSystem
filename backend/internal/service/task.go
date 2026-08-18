package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"asset-registration-management-system/backend/internal/model"

	"gorm.io/gorm"
)

type TaskHandler func(context.Context, json.RawMessage) (interface{}, error)

type TaskManager struct {
	db       *gorm.DB
	mu       sync.RWMutex
	handlers map[string]TaskHandler
}

func NewTaskManager(db *gorm.DB) *TaskManager {
	return &TaskManager{db: db, handlers: map[string]TaskHandler{}}
}

func (m *TaskManager) Register(kind string, handler TaskHandler) {
	m.mu.Lock()
	m.handlers[kind] = handler
	m.mu.Unlock()
}

func (m *TaskManager) RegisteredKinds() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	kinds := make([]string, 0, len(m.handlers))
	for kind := range m.handlers {
		kinds = append(kinds, kind)
	}
	return kinds
}

func (m *TaskManager) RecoverInterrupted() error {
	return m.db.Model(&model.BackgroundTask{}).
		Where("status = ?", model.BackgroundTaskRunning).
		Updates(map[string]interface{}{
			"status": model.BackgroundTaskQueued,
			"error":  "进程中断，任务等待恢复",
		}).Error
}

func (m *TaskManager) Run(ctx context.Context, kind, source, uniqueKey string, payload interface{}) (model.BackgroundTask, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return model.BackgroundTask{}, err
	}
	var task model.BackgroundTask
	err = m.db.Where("unique_key = ?", uniqueKey).First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		task = model.BackgroundTask{
			Kind: kind, Source: source, UniqueKey: uniqueKey,
			Status: model.BackgroundTaskQueued, Payload: string(raw),
			MaxAttempts: 3, ScheduledAt: time.Now().UTC(),
		}
		if err := m.db.Create(&task).Error; err != nil {
			if findErr := m.db.Where("unique_key = ?", uniqueKey).First(&task).Error; findErr != nil {
				return task, err
			}
		}
	} else if err != nil {
		return task, err
	}
	if task.Status == model.BackgroundTaskSucceeded || task.Status == model.BackgroundTaskRunning {
		return task, nil
	}
	if task.Status == model.BackgroundTaskFailed && task.Attempts >= task.MaxAttempts {
		return task, fmt.Errorf("task %d exhausted retries", task.ID)
	}
	return m.execute(ctx, task)
}

func (m *TaskManager) execute(ctx context.Context, task model.BackgroundTask) (model.BackgroundTask, error) {
	m.mu.RLock()
	handler := m.handlers[task.Kind]
	m.mu.RUnlock()
	if handler == nil {
		return task, fmt.Errorf("task handler is not registered: %s", task.Kind)
	}
	now := time.Now().UTC()
	result := m.db.Model(&model.BackgroundTask{}).
		Where("id = ? AND status IN ?", task.ID, []model.BackgroundTaskStatus{
			model.BackgroundTaskQueued, model.BackgroundTaskFailed,
		}).
		Updates(map[string]interface{}{
			"status":      model.BackgroundTaskRunning,
			"started_at":  now,
			"finished_at": nil,
			"attempts":    gorm.Expr("attempts + 1"),
			"error":       "",
		})
	if result.Error != nil {
		return task, result.Error
	}
	if result.RowsAffected == 0 {
		if err := m.db.First(&task, task.ID).Error; err != nil {
			return task, err
		}
		return task, nil
	}
	if err := m.db.First(&task, task.ID).Error; err != nil {
		return task, err
	}
	output, runErr := handler(ctx, json.RawMessage(task.Payload))
	finished := time.Now().UTC()
	updates := map[string]interface{}{"finished_at": finished}
	if runErr != nil {
		updates["status"] = model.BackgroundTaskFailed
		updates["error"] = runErr.Error()
	} else {
		rawOutput, _ := json.Marshal(output)
		updates["status"] = model.BackgroundTaskSucceeded
		updates["result"] = string(rawOutput)
		updates["error"] = ""
	}
	if err := m.db.Model(&task).Updates(updates).Error; err != nil {
		return task, err
	}
	if err := m.db.First(&task, task.ID).Error; err != nil {
		return task, err
	}
	return task, runErr
}

func (m *TaskManager) ResumeKind(ctx context.Context, kind string) {
	var tasks []model.BackgroundTask
	if err := m.db.Where("kind = ? AND status = ? AND attempts < max_attempts", kind, model.BackgroundTaskQueued).
		Order("scheduled_at ASC").Find(&tasks).Error; err != nil {
		return
	}
	for i := range tasks {
		task := tasks[i]
		go func() {
			_, _ = m.execute(ctx, task)
		}()
	}
}

func (m *TaskManager) List(kind, status string, page, pageSize int) ([]model.BackgroundTask, int64, error) {
	query := m.db.Model(&model.BackgroundTask{})
	if kind != "" {
		query = query.Where("kind = ?", kind)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var tasks []model.BackgroundTask
	err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&tasks).Error
	return tasks, total, err
}

func (m *TaskManager) Get(id uint) (model.BackgroundTask, error) {
	var task model.BackgroundTask
	return task, m.db.First(&task, id).Error
}

func (m *TaskManager) Retry(ctx context.Context, id uint) (model.BackgroundTask, error) {
	task, err := m.Get(id)
	if err != nil {
		return task, err
	}
	if task.Status != model.BackgroundTaskFailed {
		return task, errors.New("only failed tasks can be retried")
	}
	updates := map[string]interface{}{
		"status":          model.BackgroundTaskQueued,
		"acknowledged_at": nil,
		"acknowledged_by": nil,
	}
	if task.Attempts >= task.MaxAttempts {
		updates["max_attempts"] = task.Attempts + 1
	}
	if err := m.db.Model(&task).Updates(updates).Error; err != nil {
		return task, err
	}
	task.Status = model.BackgroundTaskQueued
	return m.execute(ctx, task)
}

func (m *TaskManager) Acknowledge(id, userID uint) error {
	now := time.Now().UTC()
	return m.db.Model(&model.BackgroundTask{}).Where("id = ?", id).
		Updates(map[string]interface{}{"acknowledged_at": now, "acknowledged_by": userID}).Error
}
