package main

import (
	"log"

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
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
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

	router := httpapi.NewRouter(httpapi.Dependencies{
		Config: cfg,
		DB:     db,
		Roles:  model.AllRoles(),
	})

	discoverySvc := service.NewDiscoveryService(db, cfg)
	scheduler := service.NewDiscoveryScheduler(discoverySvc)
	scheduler.Start()
	defer scheduler.Stop()

	log.Printf("asset registration management backend listening on %s", cfg.HTTP.Addr)
	if err := router.Run(cfg.HTTP.Addr); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
