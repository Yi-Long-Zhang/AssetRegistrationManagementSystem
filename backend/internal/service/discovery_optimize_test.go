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

// recordingRunner 记录每次调用的参数，可注入序列化输出。
type recordingRunner struct {
	calls [][]string
	outs  [][]byte
}

func (r *recordingRunner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string(nil), args...))
	if len(r.outs) > 0 {
		out := r.outs[0]
		if len(r.outs) > 1 {
			r.outs = r.outs[1:]
		}
		return out, nil
	}
	return []byte(sampleNmapXML), nil
}

func TestExpandTargets(t *testing.T) {
	cases := []struct {
		targets string
		max     int
		want    int
		wantErr bool
	}{
		{"192.168.1.0/30", 1024, 4, false},
		{"192.168.1.1", 1024, 1, false},
		{"192.168.1.1, 10.0.0.5", 1024, 2, false},
		{"192.168.1.0/24", 1024, 256, false},
		{"192.168.1.0/24", 10, 0, true}, // 超 maxHosts
		{"bad-target", 1024, 0, true},
	}
	for _, c := range cases {
		ips, err := expandTargets(c.targets, c.max)
		if (err != nil) != c.wantErr {
			t.Errorf("expandTargets(%q, %d) err = %v, wantErr = %v", c.targets, c.max, err, c.wantErr)
			continue
		}
		if !c.wantErr && len(ips) != c.want {
			t.Errorf("expandTargets(%q) got %d ips, want %d", c.targets, len(ips), c.want)
		}
	}
}

func TestTwoPhaseScanCalls(t *testing.T) {
	rule := model.DiscoveryRule{Targets: "192.168.1.0/24", Ports: "22,80", ProbePorts: "445"}
	cfg := config.DiscoveryConfig{
		DefaultPorts: "22,80,443", ProbePorts: "22,80,443,445",
		ScanChunkSize: 128, MaxParallelScans: 4, ScanTimeoutSec: 30,
	}
	rec := &recordingRunner{}
	results, err := Scan(context.Background(), rec, "nmap", rule, cfg)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	// /24 = 256 台 → 探活分 2 片（ScanChunkSize 128）+ 详扫 1 次 = 3 次调用
	if len(rec.calls) != 3 {
		t.Fatalf("expected 3 nmap calls (probe x2 chunks + detail), got %d: %v", len(rec.calls), rec.calls)
	}
	for i := 0; i < 2; i++ {
		probe := rec.calls[i]
		if !containsArg(probe, "-p") || !containsArg(probe, "445") {
			t.Fatalf("probe call %d should use rule probe ports 445: %v", i, probe)
		}
	}
	detail := rec.calls[2]
	if !containsArg(detail, "-p") || !containsArg(detail, "22,80") {
		t.Fatalf("detail call should use rule ports 22,80: %v", detail)
	}
	// 详扫目标应为探活发现的 up 主机（sampleNmapXML 里 2 台）
	if !containsArg(detail, "10.0.0.5") || !containsArg(detail, "192.168.1.10") {
		t.Fatalf("detail call should target up hosts only: %v", detail)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestSingleTargetSkipsProbe(t *testing.T) {
	rule := model.DiscoveryRule{Targets: "127.0.0.1", Ports: "22,80"}
	cfg := config.DiscoveryConfig{DefaultPorts: "22,80,443", ScanTimeoutSec: 30}
	rec := &recordingRunner{}
	_, err := Scan(context.Background(), rec, "nmap", rule, cfg)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if len(rec.calls) != 1 {
		t.Fatalf("single target should make exactly 1 call, got %d", len(rec.calls))
	}
	call := rec.calls[0]
	if containsArg(call, "445") || containsArg(call, "22,80,443,445") {
		t.Fatalf("single target should not run probe phase: %v", call)
	}
}

func TestProbeFiltersDownHosts(t *testing.T) {
	// 探活 XML：1 up + 1 down；详扫应只含 up 主机
	probeXML := `<nmaprun>
  <host><status state="up"/><address addr="10.0.0.5" addrtype="ipv4"/></host>
  <host><status state="down"/><address addr="10.0.0.6" addrtype="ipv4"/></host>
</nmaprun>`
	detailXML := `<nmaprun>
  <host><status state="up"/><address addr="10.0.0.5" addrtype="ipv4"/><ports><port protocol="tcp" portid="22"><state state="open"/></port></ports></host>
</nmaprun>`
	rec := &recordingRunner{outs: [][]byte{[]byte(probeXML), []byte(detailXML)}}
	// 目标缩小为 /29（8 台 < ScanChunkSize），探活单片 → 共 2 次调用，顺序确定
	rule := model.DiscoveryRule{Targets: "10.0.0.0/29", Ports: "22"}
	cfg := config.DiscoveryConfig{DefaultPorts: "22", ProbePorts: "445", ScanTimeoutSec: 30}
	results, err := Scan(context.Background(), rec, "nmap", rule, cfg)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if len(rec.calls) != 2 {
		t.Fatalf("expected 2 calls (probe + detail), got %d", len(rec.calls))
	}
	if len(results) != 1 || results[0].IP != "10.0.0.5" {
		t.Fatalf("expected only up host 10.0.0.5, got %+v", results)
	}
	if !containsArg(rec.calls[1], "10.0.0.5") || containsArg(rec.calls[1], "10.0.0.6") {
		t.Fatalf("detail call should only include up host: %v", rec.calls[1])
	}
}

func TestNormalizeMAC(t *testing.T) {
	if normalizeMAC("AA:BB:CC:DD:EE:FF") != "aabbccddeeff" {
		t.Fatalf("unexpected: %s", normalizeMAC("AA:BB:CC:DD:EE:FF"))
	}
	if normalizeMAC("aa-bb-cc-dd-ee-ff") != "aabbccddeeff" {
		t.Fatalf("dash form failed: %s", normalizeMAC("aa-bb-cc-dd-ee-ff"))
	}
	if normalizeMAC("") != "" {
		t.Fatalf("empty should stay empty")
	}
}

func TestAssetIPs(t *testing.T) {
	a := &model.Asset{IP: "10.0.0.5", ManagementIP: "10.0.0.5", AdditionalIPs: "10.0.0.6, 10.0.0.7"}
	ips := assetIPs(a)
	if len(ips) != 3 {
		t.Fatalf("expected 3 unique ips, got %v", ips)
	}
	if !assetSeen(a, map[string]bool{"10.0.0.7": true}) {
		t.Fatal("assetSeen should match additional ip")
	}
}

// reconcile 集成测试（内存 sqlite）：MAC 匹配 + 多 IP 命中 + 离线联动
func TestReconcileMACAndMultiIP(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Asset{}, &model.DiscoveryRun{}, &model.DiscoveredHost{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now()
	asset := model.Asset{
		IP: "10.0.0.5", MACAddress: "AA:BB:CC:DD:EE:FF",
		AdditionalIPs: "10.0.0.6, 10.0.0.7",
		OnlineStatus:  model.AssetOnlineStatusOnline, LastSeenAt: &now,
	}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	svc := &DiscoveryService{DB: db}
	run := &model.DiscoveryRun{ID: 1, RuleID: 1}

	// 场景 1：IP 未命中主 IP（10.0.0.6 关联 IP）→ 应匹配到资产，且不判离线
	results1 := []ScanResult{{IP: "10.0.0.6", Status: "up", OpenPorts: []string{"80/tcp"}}}
	if err := svc.reconcile(run, results1, now); err != nil {
		t.Fatalf("reconcile 1: %v", err)
	}
	if len(run.Hosts) == 0 {
		t.Fatalf("no hosts saved")
	}
	if run.Hosts[0].MatchedAssetID == nil || *run.Hosts[0].MatchedAssetID != asset.ID {
		t.Fatalf("multi-IP match failed: %+v", run.Hosts[0])
	}
	if run.OfflineCount != 0 {
		t.Fatalf("asset should not be offline (multi-IP seen): offline=%d", run.OfflineCount)
	}

	// 场景 2：IP 不同但 MAC 匹配 → 应通过 MAC 命中资产
	results2 := []ScanResult{{IP: "192.168.9.9", MAC: "aa:bb:cc:dd:ee:ff", Status: "up"}}
	run2 := &model.DiscoveryRun{ID: 2, RuleID: 1}
	if err := svc.reconcile(run2, results2, now); err != nil {
		t.Fatalf("reconcile 2: %v", err)
	}
	if run2.Hosts[0].MatchedAssetID == nil || *run2.Hosts[0].MatchedAssetID != asset.ID {
		t.Fatalf("MAC match failed: %+v", run2.Hosts[0])
	}
}

func containsArg(args []string, want string) bool {
	for _, a := range args {
		if strings.Contains(a, want) {
			return true
		}
	}
	return false
}
