package service

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"asset-registration-management-system/backend/internal/config"
	"asset-registration-management-system/backend/internal/model"
)

// NmapRunner 抽象 nmap 命令执行，便于测试注入 fake 实现。
type NmapRunner interface {
	Run(ctx context.Context, bin string, args ...string) ([]byte, error)
}

// execNmapRunner 通过 exec 调用真实 nmap。
type execNmapRunner struct{}

// Run 执行 nmap 并返回 stdout 输出；nmap 以非零码退出时返回错误。
func (execNmapRunner) Run(ctx context.Context, bin string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(ee.Stderr))
		}
		if stderr == "" {
			return out, fmt.Errorf("nmap execution failed: %v", err)
		}
		return out, fmt.Errorf("nmap execution failed: %v: %s", err, stderr)
	}
	return out, nil
}

// platformNmapDir 返回 tools/nmap/ 下各平台的子目录名。
func platformNmapDir() string {
	switch runtime.GOOS {
	case "windows":
		return "windows"
	case "linux":
		return "linux"
	case "darwin":
		return "macos"
	}
	return ""
}

func nmapExeName() string {
	if runtime.GOOS == "windows" {
		return "nmap.exe"
	}
	return "nmap"
}

// ResolveNmapBin 按优先级探测 nmap 可执行文件：
// 1. 配置指定路径（非空且存在）
// 2. 仓库内便携目录 tools/nmap/<platform>/（兼容 backend/ 子目录运行）
// 3. PATH 中的 nmap
// 均未找到时返回带安装提示的错误。
func ResolveNmapBin(configured string) (string, error) {
	if configured != "" {
		if info, err := os.Stat(configured); err == nil && !info.IsDir() {
			return configured, nil
		}
		return "", fmt.Errorf("configured nmap binary not found: %s", configured)
	}
	if dir := platformNmapDir(); dir != "" {
		for _, base := range []string{"", ".."} {
			// 直接位置：tools/nmap/<platform>/nmap(.exe)
			p := filepath.Join(base, "tools", "nmap", dir, nmapExeName())
			if info, err := os.Stat(p); err == nil && !info.IsDir() {
				return absPath(p), nil
			}
			// 版本子目录（如 nmap-7.92/）：tools/nmap/<platform>/*/nmap(.exe)
			pattern := filepath.Join(base, "tools", "nmap", dir, "*", nmapExeName())
			if matches, err := filepath.Glob(pattern); err == nil {
				for _, m := range matches {
					if info, err := os.Stat(m); err == nil && !info.IsDir() {
						return absPath(m), nil
					}
				}
			}
		}
	}
	if p, err := exec.LookPath(nmapExeName()); err == nil {
		return p, nil
	}
	hint := "sh"
	if runtime.GOOS == "windows" {
		hint = "ps1"
	}
	return "", fmt.Errorf("nmap not found: run scripts/setup-nmap.%s to install it, or set discovery.nmap_bin", hint)
}

// ScanResult 单台主机的扫描结果（与 nmap XML 解耦）。
type ScanResult struct {
	IP        string   `json:"ip"`
	Hostname  string   `json:"hostname"`
	Status    string   `json:"status"` // up / down
	OpenPorts []string `json:"openPorts"`
	Services  []string `json:"services"`
	OS        string   `json:"os"`
}

// nmap XML 解析结构（-oX - 输出）
type nmapRun struct {
	Hosts []nmapHost `xml:"host"`
}

type nmapHost struct {
	Status    nmapStatus    `xml:"status"`
	Addresses []nmapAddress `xml:"address"`
	Hostnames struct {
		Names []nmapHostname `xml:"hostname"`
	} `xml:"hostnames"`
	Ports struct {
		Ports []nmapPort `xml:"port"`
	} `xml:"ports"`
	OS struct {
		Matches []nmapOSMatch `xml:"osmatch"`
	} `xml:"os"`
}

type nmapStatus struct {
	State string `xml:"state,attr"`
}

type nmapAddress struct {
	Addr     string `xml:"addr,attr"`
	AddrType string `xml:"addrtype,attr"`
}

type nmapHostname struct {
	Name string `xml:"name,attr"`
}

type nmapPort struct {
	Protocol string        `xml:"protocol,attr"`
	PortID   string        `xml:"portid,attr"`
	State    nmapPortState `xml:"state"`
	Service  nmapService   `xml:"service"`
}

type nmapPortState struct {
	State string `xml:"state,attr"`
}

type nmapService struct {
	Name    string `xml:"name,attr"`
	Product string `xml:"product,attr"`
	Version string `xml:"version,attr"`
}

type nmapOSMatch struct {
	Name string `xml:"name,attr"`
}

// ParseNmapXML 将 nmap -oX 输出解析为有序的 ScanResult 列表（按 IP 排序）。
func ParseNmapXML(data []byte) ([]ScanResult, error) {
	var run nmapRun
	if err := xml.Unmarshal(data, &run); err != nil {
		return nil, fmt.Errorf("parse nmap xml: %w", err)
	}
	results := make([]ScanResult, 0, len(run.Hosts))
	for _, h := range run.Hosts {
		res := ScanResult{Status: h.Status.State}
		for _, a := range h.Addresses {
			if a.AddrType == "ipv4" && res.IP == "" {
				res.IP = a.Addr
			}
		}
		if res.IP == "" && len(h.Addresses) > 0 {
			res.IP = h.Addresses[0].Addr
		}
		if len(h.Hostnames.Names) > 0 {
			res.Hostname = h.Hostnames.Names[0].Name
		}
		if len(h.OS.Matches) > 0 {
			res.OS = h.OS.Matches[0].Name
		}
		var ports, services []string
		for _, p := range h.Ports.Ports {
			if p.State.State != "open" {
				continue
			}
			ports = append(ports, p.PortID+"/"+p.Protocol)
			s := p.PortID + "/" + p.Protocol + ": " + p.Service.Name
			if p.Service.Product != "" {
				s += " " + p.Service.Product
			}
			if p.Service.Version != "" {
				s += " " + p.Service.Version
			}
			services = append(services, s)
		}
		sort.Strings(ports)
		sort.Strings(services)
		res.OpenPorts = ports
		res.Services = services
		results = append(results, res)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].IP < results[j].IP })
	return results, nil
}

func absPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// BuildNmapArgs 依据规则与全局配置组装 nmap 参数（切片传参，杜绝 shell 拼接注入）。
func BuildNmapArgs(rule model.DiscoveryRule, cfg config.DiscoveryConfig) []string {
	args := []string{"-oX", "-"}
	// -Pn：跳过主机发现直接端口扫描（服务器常禁 ping，无 Npcap 环境下回环探测也会误判 down）
	args = append(args, "-Pn")
	if rule.ServiceDetect {
		args = append(args, "-sV")
	}
	ports := strings.TrimSpace(rule.Ports)
	if ports == "" {
		ports = strings.TrimSpace(cfg.DefaultPorts)
	}
	if ports != "" {
		args = append(args, "-p", ports)
	}
	args = append(args, splitTargets(rule.Targets)...)
	return args
}

// splitTargets 将规则目标（CIDR/IP，逗号或换行分隔）拆为 nmap 目标参数列表。
func splitTargets(targets string) []string {
	var out []string
	for _, t := range strings.FieldsFunc(targets, func(r rune) bool {
		return r == ',' || r == '\n' || r == ' ' || r == '\r' || r == '\t'
	}) {
		t = strings.TrimSpace(t)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// ValidateTargets 校验目标格式（IPv4 或 CIDR），返回错误描述；空目标报错。
func ValidateTargets(targets string) error {
	parts := splitTargets(targets)
	if len(parts) == 0 {
		return fmt.Errorf("targets must contain at least one IP or CIDR")
	}
	seen := map[string]bool{}
	for _, p := range parts {
		if seen[p] {
			return fmt.Errorf("duplicate target: %s", p)
		}
		seen[p] = true
		// 简单合法性检查：IPv4 或 IPv4/CIDR
		ipPart := p
		if strings.Contains(p, "/") {
			parts2 := strings.SplitN(p, "/", 2)
			ipPart = parts2[0]
			prefix, err := strconv.Atoi(parts2[1])
			if err != nil || prefix < 0 || prefix > 32 {
				return fmt.Errorf("invalid CIDR: %s", p)
			}
		}
		octets := strings.Split(ipPart, ".")
		if len(octets) != 4 {
			return fmt.Errorf("invalid IP or CIDR: %s", p)
		}
		for _, o := range octets {
			n, err := strconv.Atoi(o)
			if err != nil || n < 0 || n > 255 {
				return fmt.Errorf("invalid IP or CIDR: %s", p)
			}
		}
	}
	return nil
}

// ValidatePorts 校验端口列表格式（数字或数字-数字，逗号分隔），空表示使用默认端口。
func ValidatePorts(ports string) error {
	if strings.TrimSpace(ports) == "" {
		return nil
	}
	for _, item := range strings.Split(ports, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			return fmt.Errorf("invalid ports: %q", ports)
		}
		if strings.Contains(item, "-") {
			bounds := strings.SplitN(item, "-", 2)
			for _, b := range bounds {
				n, err := strconv.Atoi(strings.TrimSpace(b))
				if err != nil || n < 1 || n > 65535 {
					return fmt.Errorf("invalid port range: %s", item)
				}
			}
			continue
		}
		n, err := strconv.Atoi(item)
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("invalid port: %s", item)
		}
	}
	return nil
}

// Scan 执行一次 nmap 扫描并返回解析后的结果；nmap 缺失、超时或 XML 解析失败时返回错误。
// runner 为 nil 时使用真实 exec 实现。
func Scan(ctx context.Context, runner NmapRunner, bin string, rule model.DiscoveryRule, cfg config.DiscoveryConfig) ([]ScanResult, error) {
	if runner == nil {
		runner = execNmapRunner{}
	}
	timeout := time.Duration(cfg.ScanTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 300 * time.Second
	}
	scanCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := BuildNmapArgs(rule, cfg)
	out, err := runner.Run(scanCtx, bin, args...)
	if err != nil {
		return nil, err
	}
	return ParseNmapXML(out)
}
