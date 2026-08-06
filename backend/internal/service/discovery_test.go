package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"asset-registration-management-system/backend/internal/config"
	"asset-registration-management-system/backend/internal/model"
)

// fakeRunner 注入固定输出/错误的 NmapRunner 测试替身。
type fakeRunner struct {
	out []byte
	err error
}

func (f fakeRunner) Run(_ context.Context, _ string, _ ...string) ([]byte, error) {
	return f.out, f.err
}

const sampleNmapXML = `<?xml version="1.0"?>
<nmaprun scanner="nmap" start="1700000000">
  <host starttime="1700000000" endtime="1700000001">
    <status state="up" reason="syn-ack"/>
    <address addr="192.168.1.10" addrtype="ipv4"/>
    <hostnames><hostname name="web-01.internal" type="PTR"/></hostnames>
    <ports>
      <port protocol="tcp" portid="80">
        <state state="open"/>
        <service name="http" product="nginx" version="1.18.0"/>
      </port>
      <port protocol="tcp" portid="443">
        <state state="open"/>
        <service name="https" product="nginx"/>
      </port>
      <port protocol="tcp" portid="22">
        <state state="filtered"/>
      </port>
    </ports>
    <os><osmatch name="Linux 4.15"/></os>
  </host>
  <host starttime="1700000000" endtime="1700000001">
    <status state="up" reason="syn-ack"/>
    <address addr="10.0.0.5" addrtype="ipv4"/>
    <hostnames><hostname name="db-01.internal" type="PTR"/></hostnames>
    <ports>
      <port protocol="tcp" portid="5432">
        <state state="open"/>
        <service name="postgresql" product="PostgreSQL DB" version="13.1"/>
      </port>
    </ports>
  </host>
</nmaprun>`

func TestParseNmapXML(t *testing.T) {
	results, err := ParseNmapXML([]byte(sampleNmapXML))
	if err != nil {
		t.Fatalf("ParseNmapXML error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(results))
	}
	// 排序：10.0.0.5 < 192.168.1.10
	if results[0].IP != "10.0.0.5" || results[1].IP != "192.168.1.10" {
		t.Fatalf("hosts not sorted by IP: %+v", results)
	}
	db := results[0]
	if db.Hostname != "db-01.internal" || db.Status != "up" {
		t.Fatalf("unexpected db host: %+v", db)
	}
	if len(db.OpenPorts) != 1 || db.OpenPorts[0] != "5432/tcp" {
		t.Fatalf("unexpected db ports: %+v", db.OpenPorts)
	}
	web := results[1]
	if web.OS != "Linux 4.15" {
		t.Fatalf("unexpected os: %q", web.OS)
	}
	if len(web.OpenPorts) != 2 {
		t.Fatalf("expected 2 open ports (filtered excluded), got %+v", web.OpenPorts)
	}
	if web.OpenPorts[0] != "443/tcp" || web.OpenPorts[1] != "80/tcp" {
		t.Fatalf("ports not sorted: %+v", web.OpenPorts)
	}
	if len(web.Services) != 2 || !strings.Contains(web.Services[0], "nginx") {
		t.Fatalf("unexpected services: %+v", web.Services)
	}
}

func TestParseNmapXMLInvalid(t *testing.T) {
	if _, err := ParseNmapXML([]byte("<nmaprun><broken")); err == nil {
		t.Fatal("expected error for invalid xml")
	}
}

func TestParseNmapXMLDownHost(t *testing.T) {
	xml := `<nmaprun>
  <host><status state="down"/><address addr="192.168.1.99" addrtype="ipv4"/></host>
</nmaprun>`
	results, err := ParseNmapXML([]byte(xml))
	if err != nil {
		t.Fatalf("ParseNmapXML error: %v", err)
	}
	if len(results) != 1 || results[0].Status != "down" || results[0].IP != "192.168.1.99" {
		t.Fatalf("unexpected down host result: %+v", results)
	}
}

func TestBuildNmapArgs(t *testing.T) {
	cfg := config.DiscoveryConfig{DefaultPorts: "22,80,443"}

	// 规则指定端口 + 服务探测
	rule := model.DiscoveryRule{
		Targets:       "192.168.1.0/24, 10.0.0.5",
		Ports:         "22,3389",
		ServiceDetect: true,
	}
	args := BuildNmapArgs(rule, cfg)
	want := []string{"-oX", "-", "-sV", "-p", "22,3389", "192.168.1.0/24", "10.0.0.5"}
	if len(args) != len(want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args = %v, want %v", args, want)
		}
	}

	// 规则未指定端口 → 使用默认端口
	rule2 := model.DiscoveryRule{Targets: "192.168.1.1"}
	args2 := BuildNmapArgs(rule2, cfg)
	if len(args2) != 5 || args2[2] != "-p" || args2[3] != "22,80,443" {
		t.Fatalf("default ports not applied: %v", args2)
	}
}

func TestValidateTargets(t *testing.T) {
	cases := []struct {
		targets string
		wantErr bool
	}{
		{"192.168.1.1", false},
		{"192.168.1.0/24, 10.0.0.1", false},
		{"", true},
		{"999.1.1.1", true},
		{"192.168.1.0/33", true},
		{"not-an-ip", true},
		{"192.168.1.1,192.168.1.1", true}, // 重复
	}
	for _, c := range cases {
		err := ValidateTargets(c.targets)
		if (err != nil) != c.wantErr {
			t.Errorf("ValidateTargets(%q) err = %v, wantErr = %v", c.targets, err, c.wantErr)
		}
	}
}

func TestValidatePorts(t *testing.T) {
	if err := ValidatePorts("22,80,443"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidatePorts("1000-2000"); err != nil {
		t.Errorf("unexpected range error: %v", err)
	}
	if err := ValidatePorts(""); err != nil {
		t.Errorf("empty ports should be valid: %v", err)
	}
	for _, bad := range []string{"0", "70000", "abc", "22,,80", "22-"} {
		if err := ValidatePorts(bad); err == nil {
			t.Errorf("ValidatePorts(%q) should error", bad)
		}
	}
}

func TestResolveNmapBinConfiguredMissing(t *testing.T) {
	if _, err := ResolveNmapBin("C:/nonexistent/nmap.exe"); err == nil {
		t.Fatal("expected error for missing configured binary")
	}
}

func TestScanWithFakeRunner(t *testing.T) {
	fake := fakeRunner{out: []byte(sampleNmapXML)}
	rule := model.DiscoveryRule{Targets: "192.168.1.0/24"}
	cfg := config.DiscoveryConfig{DefaultPorts: "22,80,443", ScanTimeoutSec: 30}
	results, err := Scan(context.Background(), fake, "nmap", rule, cfg)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestScanFakeRunnerError(t *testing.T) {
	fake := fakeRunner{err: errors.New("nmap not found")}
	rule := model.DiscoveryRule{Targets: "192.168.1.1"}
	cfg := config.DiscoveryConfig{ScanTimeoutSec: 30}
	_, err := Scan(context.Background(), fake, "nmap", rule, cfg)
	if err == nil {
		t.Fatal("expected error propagation")
	}
}

func TestScanNoRunnerUsesExec(t *testing.T) {
	// 真实 exec runner 在无 nmap 环境下应返回错误（本机无 nmap），验证缺省路径提示
	rule := model.DiscoveryRule{Targets: "127.0.0.1"}
	cfg := config.DiscoveryConfig{ScanTimeoutSec: 5}
	_, err := Scan(context.Background(), nil, "definitely-not-a-real-nmap-binary", rule, cfg)
	if err == nil {
		t.Fatal("expected error when binary does not exist")
	}
}
