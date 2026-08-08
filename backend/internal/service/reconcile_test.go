package service

import (
	"strings"
	"testing"

	"asset-registration-management-system/backend/internal/model"
)

func TestClassifyChangeChanged(t *testing.T) {
	asset := &model.Asset{
		Hostname:  "web-01",
		IP:        "192.168.1.10",
		OpenPorts: "80,443",
		OS:        "Linux",
	}
	r := ScanResult{
		IP:        "192.168.1.10",
		Hostname:  "web-01",
		OpenPorts: []string{"80/tcp", "443/tcp", "22/tcp"},
		OS:        "Linux",
	}
	change, diff, risk := classifyChange(asset, r)
	if change != model.DiscoveryChangeChanged {
		t.Fatalf("expected changed, got %s (diff: %s)", change, diff)
	}
	if !strings.Contains(diff, "开放端口") {
		t.Fatalf("diff should mention ports: %s", diff)
	}
	if !strings.Contains(diff, "新增 22") {
		t.Fatalf("diff should list added port 22: %s", diff)
	}
	if risk != model.ChangeRiskLow {
		t.Fatalf("only added port should be low risk, got %s", risk)
	}
}

func TestClassifyChangeOnline(t *testing.T) {
	asset := &model.Asset{
		Hostname:     "db-01",
		IP:           "10.0.0.5",
		OpenPorts:    "5432",
		OnlineStatus: model.AssetOnlineStatusUnknown,
	}
	r := ScanResult{
		IP:        "10.0.0.5",
		Hostname:  "db-01",
		OpenPorts: []string{"5432/tcp"},
	}
	change, _, _ := classifyChange(asset, r)
	if change != model.DiscoveryChangeOnline {
		t.Fatalf("expected online, got %s", change)
	}
}

func TestClassifyChangeNone(t *testing.T) {
	asset := &model.Asset{
		Hostname:     "web-01",
		IP:           "192.168.1.10",
		OpenPorts:    "80/tcp,443/tcp",
		OS:           "Linux 4.15",
		OnlineStatus: model.AssetOnlineStatusOnline,
	}
	r := ScanResult{
		IP:        "192.168.1.10",
		Hostname:  "web-01",
		OpenPorts: []string{"443/tcp", "80/tcp"},
		OS:        "Linux 4.15",
	}
	change, _, _ := classifyChange(asset, r)
	if change != model.DiscoveryChangeNone {
		t.Fatalf("expected none, got %s", change)
	}
}

func TestClassifyChangeHostnameContains(t *testing.T) {
	// 反向 DNS 短名包含于资产全名时不视为变更（避免误报）
	asset := &model.Asset{Hostname: "web-01.internal", IP: "192.168.1.10", OnlineStatus: model.AssetOnlineStatusOnline}
	r := ScanResult{IP: "192.168.1.10", Hostname: "web-01"}
	change, _, _ := classifyChange(asset, r)
	if change != model.DiscoveryChangeNone {
		t.Fatalf("expected none for contained hostname, got %s", change)
	}
}

func TestPortSetNormalization(t *testing.T) {
	a := portSet([]string{"80/tcp", "443", ""})
	b := portSet([]string{"443/tcp", "80"})
	if !equalSet(a, b) {
		t.Fatalf("port sets should be equal: %v vs %v", a, b)
	}
	c := portSet([]string{"22"})
	if equalSet(a, c) {
		t.Fatal("port sets should differ")
	}
}

func TestAssetNoFromIP(t *testing.T) {
	if assetNoFromIP("192.168.1.10") != "IP-192-168-1-10" {
		t.Fatalf("unexpected asset no: %s", assetNoFromIP("192.168.1.10"))
	}
	if assetNoFromIP("") != "" {
		t.Fatal("empty ip should yield empty asset no")
	}
}

func TestDiffFields(t *testing.T) {
	prev := assetFieldMap(&model.Asset{
		Hostname:  "old",
		IP:        "192.168.1.1",
		OpenPorts: "80",
		OS:        "Linux",
	})
	cur := assetFieldMap(&model.Asset{
		Hostname:  "new",
		IP:        "192.168.1.1",
		OpenPorts: "80,443",
		OS:        "",
	})
	lines, summary := diffFields(prev, cur)
	if len(lines) != 3 {
		t.Fatalf("expected 3 diff lines, got %d: %v", len(lines), lines)
	}
	if !strings.Contains(summary, "hostname") || !strings.Contains(summary, "openPorts") || !strings.Contains(summary, "清空") {
		t.Fatalf("unexpected summary: %s", summary)
	}
}
