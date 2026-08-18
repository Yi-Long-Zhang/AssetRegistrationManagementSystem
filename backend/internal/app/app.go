package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"asset-registration-management-system/backend/internal/config"
	"asset-registration-management-system/backend/internal/database"
	"asset-registration-management-system/backend/internal/httpapi"
	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"

	"gorm.io/gorm"
)

type scheduler interface {
	Start()
	Stop()
}

// Application owns the process-level resources used by the HTTP service.
type Application struct {
	server     *http.Server
	db         *gorm.DB
	schedulers []scheduler
	closeOnce  sync.Once
	closeErr   error
}

// New builds the application and validates all resources required at startup.
func New(cfg config.Config) (*Application, error) {
	restoreService := service.NewFullBackupService(nil, cfg)
	restored, err := restoreService.ApplyPendingRestore()
	if err != nil {
		return nil, fmt.Errorf("apply pending restore: %w", err)
	}
	if restored {
		slog.Info("database restored from pending backup")
	}

	db, err := database.Open(cfg.Storage.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	closeDB := func() {
		if sqlDB, sqlErr := db.DB(); sqlErr == nil {
			_ = sqlDB.Close()
		}
	}
	if err := database.Migrate(db); err != nil {
		closeDB()
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	if err := database.SeedAdmin(db, cfg.Admin.Username, cfg.Admin.Password); err != nil {
		closeDB()
		return nil, fmt.Errorf("seed admin: %w", err)
	}

	taskManager := service.NewTaskManager(db)
	if err := taskManager.RecoverInterrupted(); err != nil {
		closeDB()
		return nil, fmt.Errorf("recover interrupted tasks: %w", err)
	}

	router := httpapi.NewRouter(httpapi.Dependencies{
		Config: cfg,
		DB:     db,
		Roles:  model.AllRoles(),
		Tasks:  taskManager,
	})
	discoveryService := service.NewDiscoveryService(db, cfg)
	schedulers := []scheduler{
		service.NewDiscoveryScheduler(discoveryService).WithTaskManager(taskManager),
		service.NewSLAScheduler(db, cfg.Security.ConfigEncryptionKey).WithTaskManager(taskManager),
		service.NewInspectionScheduler(db).WithTaskManager(taskManager),
		service.NewWarrantyReminderScheduler(db, cfg.Security.ConfigEncryptionKey).WithTaskManager(taskManager),
		service.NewLicenseReminderScheduler(db, cfg.Security.ConfigEncryptionKey).WithTaskManager(taskManager),
		service.NewBackupScheduler(service.NewFullBackupService(db, cfg)).WithTaskManager(taskManager),
	}

	return &Application{
		db:         db,
		schedulers: schedulers,
		server: &http.Server{
			Addr:              cfg.HTTP.Addr,
			Handler:           router,
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}, nil
}

// Run serves requests until the context is cancelled or the server fails.
func (a *Application) Run(ctx context.Context) error {
	for _, item := range a.schedulers {
		item.Start()
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- a.server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		_ = a.Close()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		shutdownErr := a.server.Shutdown(shutdownCtx)
		closeErr := a.Close()
		return errors.Join(shutdownErr, closeErr)
	}
}

// Close stops background work and closes the database. It is safe to call repeatedly.
func (a *Application) Close() error {
	a.closeOnce.Do(func() {
		for index := len(a.schedulers) - 1; index >= 0; index-- {
			a.schedulers[index].Stop()
		}
		sqlDB, err := a.db.DB()
		if err != nil {
			a.closeErr = err
			return
		}
		a.closeErr = sqlDB.Close()
	})
	return a.closeErr
}
