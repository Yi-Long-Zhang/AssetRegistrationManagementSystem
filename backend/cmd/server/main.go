package main

import (
	"log"
	"log/slog"
	"os"

	"asset-registration-management-system/backend/internal/config"
	"asset-registration-management-system/backend/internal/database"
	"asset-registration-management-system/backend/internal/httpapi"
	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"
)

// @title Asset Registration Management System API
// @version 1.0
// @description 企业内部服务器资产台账与工单流程管理系统 REST API。
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	log.SetFlags(0)
	log.SetOutput(slog.NewLogLogger(logger.Handler(), slog.LevelInfo).Writer())

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	// 处理待恢复的数据库备份（恢复操作标记后重启生效）
	restoreSvc := service.NewFullBackupService(nil, cfg)
	if restored, err := restoreSvc.ApplyPendingRestore(); err != nil {
		log.Fatalf("apply pending restore: %v", err)
	} else if restored {
		log.Printf("database restored from pending backup")
	}

	db, err := database.Open(cfg.Storage.DatabasePath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	if err := database.Migrate(db); err != nil {
		log.Fatalf("migrate database: %v", err)
	}

	if err := database.SeedAdmin(db, cfg.Admin.Username, cfg.Admin.Password); err != nil {
		log.Fatalf("seed admin: %v", err)
	}
	taskManager := service.NewTaskManager(db)
	if err := taskManager.RecoverInterrupted(); err != nil {
		log.Fatalf("recover interrupted tasks: %v", err)
	}

	router := httpapi.NewRouter(httpapi.Dependencies{
		Config: cfg,
		DB:     db,
		Roles:  model.AllRoles(),
		Tasks:  taskManager,
	})

	discoverySvc := service.NewDiscoveryService(db, cfg)
	scheduler := service.NewDiscoveryScheduler(discoverySvc).WithTaskManager(taskManager)
	scheduler.Start()
	defer scheduler.Stop()

	slaScheduler := service.NewSLAScheduler(db, cfg.Security.ConfigEncryptionKey).WithTaskManager(taskManager)
	slaScheduler.Start()
	defer slaScheduler.Stop()

	inspectionScheduler := service.NewInspectionScheduler(db).WithTaskManager(taskManager)
	inspectionScheduler.Start()
	defer inspectionScheduler.Stop()

	warrantyScheduler := service.NewWarrantyReminderScheduler(db, cfg.Security.ConfigEncryptionKey).WithTaskManager(taskManager)
	warrantyScheduler.Start()
	defer warrantyScheduler.Stop()

	licenseScheduler := service.NewLicenseReminderScheduler(db, cfg.Security.ConfigEncryptionKey).WithTaskManager(taskManager)
	licenseScheduler.Start()
	defer licenseScheduler.Stop()

	backupScheduler := service.NewBackupScheduler(service.NewFullBackupService(db, cfg)).WithTaskManager(taskManager)
	backupScheduler.Start()
	defer backupScheduler.Stop()

	log.Printf("asset registration management backend listening on %s", cfg.HTTP.Addr)
	if err := router.Run(cfg.HTTP.Addr); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
