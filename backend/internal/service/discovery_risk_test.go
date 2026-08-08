package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"asset-registration-management-system/backend/internal/config"
	"asset-registration-management-system/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// P3：端口增/减明细 + 风险分级
func TestClassifyChangePortDetailAndRisk(t *testing.T) {
	asset := &model.Asset{
		IP: "10.0.0.8", OpenPorts: "80,443", Hostname: "srv-1",
		RunningServices: "80/tcp: http", OnlineStatus: model.AssetOnlineStatusOnline,
	}
	r := ScanResult{
		IP: "10.0.0.8", Hostname: "srv-1",
		OpenPorts: []string{"80/tcp", "8080/tcp"}, // 关闭 443、新增 8080
		Services:  []string{"80/tcp: http"},
	}
	change, diff, risk := classifyChange(asset, r)
	if change != model.DiscoveryChangeChanged {
		t.Fatalf("expected changed, got %s", change)
	}
	if !strings.Contains(diff, "新增 8080") || !strings.Contains(diff, "关闭 443") {
		t.Fatalf("diff should list added/closed ports: %s", diff)
	}
	if risk != model.ChangeRiskHigh {
		t.Fatalf("port closed should be high risk, got %s", risk)
	}
}

// P3：仅端口新增 → 低风险（可自动应用）
func TestClassifyChangeLowRiskOnlyAdded(t *testing.T) {
	asset := &model.Asset{
		IP: "10.0.0.9", OpenPorts: "80", Hostname: "web-1",
		RunningServices: "80/tcp: http", OnlineStatus: model.AssetOnlineStatusOnline,
	}
	r := ScanResult{
		IP: "10.0.0.9", Hostname: "web-1",
		OpenPorts: []string{"80/tcp", "8080/tcp"}, // 仅新增 8080
		Services:  []string{"80/tcp: http"},
	}
	change, diff, risk := classifyChange(asset, r)
	if change != model.DiscoveryChangeChanged {
		t.Fatalf("expected changed, got %s", change)
	}
	if !strings.Contains(diff, "新增 8080") || strings.Contains(diff, "关闭") {
		t.Fatalf("diff mismatch: %s", diff)
	}
	if risk != model.ChangeRiskLow {
		t.Fatalf("only added port should be low risk, got %s", risk)
	}
}

// P3：服务版本变化 → 高风险
func TestClassifyChangeServiceVersionDiff(t *testing.T) {
	asset := &model.Asset{
		IP: "10.0.0.10", OpenPorts: "80", Hostname: "web-2",
		RunningServices: "80/tcp: http nginx 1.18.0", OnlineStatus: model.AssetOnlineStatusOnline,
	}
	r := ScanResult{
		IP: "10.0.0.10", Hostname: "web-2",
		OpenPorts: []string{"80/tcp"},
		Services:  []string{"80/tcp: http nginx 1.20.0"}, // 版本升级
	}
	change, diff, risk := classifyChange(asset, r)
	if change != model.DiscoveryChangeChanged {
		t.Fatalf("expected changed, got %s", change)
	}
	if !strings.Contains(diff, "服务") {
		t.Fatalf("diff should mention service change: %s", diff)
	}
	if risk != model.ChangeRiskHigh {
		t.Fatalf("service change should be high risk, got %s", risk)
	}
}

// P3：主机名变化 → 高风险
func TestClassifyChangeHostnameHighRisk(t *testing.T) {
	asset := &model.Asset{
		IP: "10.0.0.11", OpenPorts: "80", Hostname: "old-name",
		OnlineStatus: model.AssetOnlineStatusOnline,
	}
	r := ScanResult{IP: "10.0.0.11", Hostname: "new-name", OpenPorts: []string{"80/tcp"}}
	change, diff, risk := classifyChange(asset, r)
	if change != model.DiscoveryChangeChanged || risk != model.ChangeRiskHigh {
		t.Fatalf("expected changed+high, got %s/%s (diff: %s)", change, risk, diff)
	}
}

// P4：跨规则确认——最近一次任意规则运行仍发现该主机 → 不判离线
func TestAbsentInLatestRunCrossRule(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.DiscoveryRun{}, &model.DiscoveredHost{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// 规则 A 的最新运行（run 3）含该 IP → 跨规则确认应返回 false（仍在线）
	runs := []model.DiscoveryRun{
		{ID: 1, RuleID: 1, Status: model.DiscoveryRunStatusSuccess},
		{ID: 2, RuleID: 2, Status: model.DiscoveryRunStatusSuccess},
		{ID: 3, RuleID: 1, Status: model.DiscoveryRunStatusSuccess},
	}
	if err := db.Create(&runs).Error; err != nil {
		t.Fatalf("create runs: %v", err)
	}
	if err := db.Create(&model.DiscoveredHost{RunID: 3, IP: "10.1.1.5", Status: "up"}).Error; err != nil {
		t.Fatalf("create host: %v", err)
	}
	absent, err := absentInLatestRun(db, "10.1.1.5", 99)
	if err != nil {
		t.Fatalf("absentInLatestRun: %v", err)
	}
	if absent {
		t.Fatal("cross-rule latest run still sees the host; should NOT be absent")
	}

	// 最近一次运行（run 3）不包含该 IP → absent
	absent2, err := absentInLatestRun(db, "10.1.1.99", 99)
	if err != nil {
		t.Fatalf("absentInLatestRun 2: %v", err)
	}
	if !absent2 {
		t.Fatal("host not in latest run should be absent")
	}
}

// P3：ExecuteRun 自动应用低风险变更（仅新增端口）
func TestExecuteRunAutoApplyLowRisk(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Asset{}, &model.DiscoveryRun{}, &model.DiscoveredHost{}, &model.AssetSnapshot{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now()
	asset := model.Asset{
		IP: "192.168.1.10", Hostname: "web-01.internal", OpenPorts: "80/tcp",
		OnlineStatus: model.AssetOnlineStatusOnline, LastSeenAt: &now,
	}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	rule := model.DiscoveryRule{
		Name: "auto", Targets: "192.168.1.10", Ports: "80",
		AutoApply: true, Enabled: true,
	}
	if err := db.Create(&rule).Error; err != nil {
		t.Fatalf("create rule: %v", err)
	}

	cfg := config.Config{}
	cfg.Discovery = config.DiscoveryConfig{
		DefaultPorts: "80", ProbePorts: "80", ScanChunkSize: 128,
		MaxHosts: 1024, OfflineAfterHours: 24,
	}
	svc := &DiscoveryService{
		DB:     db,
		Config: cfg,
		Runner: fakeRunner{out: []byte(sampleNmapXML)}, // 192.168.1.10 端口 80+443，10.0.0.5 为新增
		BinResolver: func() (string, error) {
			return "nmap", nil
		},
	}

	run, err := svc.StartRun(rule.ID, "test")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	if err := svc.ExecuteRun(context.Background(), run.ID); err != nil {
		t.Fatalf("execute run: %v", err)
	}

	// 192.168.1.10 新增 443 → 低风险 → 自动应用
	var updated model.Asset
	if err := db.First(&updated, asset.ID).Error; err != nil {
		t.Fatalf("load asset: %v", err)
	}
	if !strings.Contains(updated.OpenPorts, "443/tcp") {
		t.Fatalf("auto-apply should add 443 to asset ports: %q", updated.OpenPorts)
	}
	var host model.DiscoveredHost
	if err := db.Where("run_id = ? AND ip = ?", run.ID, "192.168.1.10").First(&host).Error; err != nil {
		t.Fatalf("load host: %v", err)
	}
	if host.ChangeRisk != model.ChangeRiskLow || !host.Applied {
		t.Fatalf("expected low-risk auto-applied host, got risk=%s applied=%v", host.ChangeRisk, host.Applied)
	}

	// 新增主机（10.0.0.5）不应被自动纳管（autoAdopt 未开）
	var newHost model.DiscoveredHost
	if err := db.Where("run_id = ? AND ip = ?", run.ID, "10.0.0.5").First(&newHost).Error; err != nil {
		t.Fatalf("load new host: %v", err)
	}
	if newHost.Adopted {
		t.Fatal("new host should not be auto-adopted without autoAdopt")
	}
}
