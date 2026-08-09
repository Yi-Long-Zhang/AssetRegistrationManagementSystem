package service

import (
	"context"
	"encoding/xml"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
	MAC       string   `json:"mac"`
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
			switch a.AddrType {
			case "ipv4":
				if res.IP == "" {
					res.IP = a.Addr
				}
			case "mac":
				if res.MAC == "" {
					res.MAC = a.Addr
				}
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
	return BuildNmapArgsFor(rule, cfg, splitTargets(rule.Targets))
}

// BuildNmapArgsFor 依据规则组装 nmap 参数，目标以传入列表为准（用于分片/两阶段扫描）。
func BuildNmapArgsFor(rule model.DiscoveryRule, cfg config.DiscoveryConfig, targets []string) []string {
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
	// 速率限制：>0 时生效，控制扫描对生产网络的影响
	if cfg.MinRate > 0 {
		args = append(args, "--min-rate", fmt.Sprintf("%d", cfg.MinRate))
	}
	if cfg.MaxRate > 0 {
		args = append(args, "--max-rate", fmt.Sprintf("%d", cfg.MaxRate))
	}
	args = append(args, targets...)
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

// Scan 执行一次发现扫描：目标展开 → 分片 → 两阶段（探活 + 详扫）并行执行。
// 单目标规则直接单次详扫（跳过探活阶段）；大网段按 ScanChunkSize 分片、MaxParallelScans 并发。
func Scan(ctx context.Context, runner NmapRunner, bin string, rule model.DiscoveryRule, cfg config.DiscoveryConfig) ([]ScanResult, error) {
	return ScanTargets(ctx, runner, bin, rule, cfg, nil)
}

// ScanTargets 执行扫描；targets 非空时使用给定主机列表（增量扫描场景，不再按规则目标展开），
// 为空时按规则 Targets 展开（含 CIDR）。
func ScanTargets(ctx context.Context, runner NmapRunner, bin string, rule model.DiscoveryRule, cfg config.DiscoveryConfig, targets []string) ([]ScanResult, error) {
	if runner == nil {
		runner = execNmapRunner{}
	}
	var hosts []string
	if len(targets) > 0 {
		hosts = targets
		if cfg.MaxHosts > 0 && len(hosts) > cfg.MaxHosts {
			hosts = hosts[:cfg.MaxHosts]
		}
	} else {
		var err error
		hosts, err = expandTargets(rule.Targets, cfg.MaxHosts)
		if err != nil {
			return nil, fmt.Errorf("expand targets: %w", err)
		}
	}
	if len(hosts) == 0 {
		return nil, fmt.Errorf("no targets to scan")
	}

	// 单目标（如单 IP 规则）：直接详扫
	if len(hosts) <= 1 {
		scanCtx, cancel := scanContext(ctx, cfg)
		defer cancel()
		out, err := runner.Run(scanCtx, bin, BuildNmapArgsFor(rule, cfg, hosts)...)
		if err != nil {
			return nil, err
		}
		return ParseNmapXML(out)
	}

	// 两阶段：先用探活端口快速定位存活主机，再对存活主机做全端口详扫
	probePorts := strings.TrimSpace(rule.ProbePorts)
	if probePorts == "" {
		probePorts = strings.TrimSpace(cfg.ProbePorts)
	}
	upHosts, err := probeAlive(ctx, runner, bin, hosts, probePorts, cfg)
	if err != nil {
		return nil, err
	}
	if len(upHosts) == 0 {
		return nil, nil
	}
	return scanHosts(ctx, runner, bin, upHosts, rule, cfg)
}

func scanContext(ctx context.Context, cfg config.DiscoveryConfig) (context.Context, context.CancelFunc) {
	timeout := time.Duration(cfg.ScanTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 300 * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}

// probeAlive 阶段一：对主机列表分片并行跑探活端口扫描，返回 up 主机 IP 列表。
func probeAlive(ctx context.Context, runner NmapRunner, bin string, hosts []string, probePorts string, cfg config.DiscoveryConfig) ([]string, error) {
	results, err := scanChunks(ctx, runner, bin, hosts, cfg, func(chunk []string) []string {
		args := []string{"-oX", "-", "-Pn", "-p", probePorts}
		return append(args, chunk...)
	})
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var up []string
	for _, r := range results {
		if r.Status == "up" && r.IP != "" && !seen[r.IP] {
			seen[r.IP] = true
			up = append(up, r.IP)
		}
	}
	sort.Strings(up)
	return up, nil
}

// scanHosts 阶段二：对存活主机分片并行跑全端口详扫。
func scanHosts(ctx context.Context, runner NmapRunner, bin string, hosts []string, rule model.DiscoveryRule, cfg config.DiscoveryConfig) ([]ScanResult, error) {
	return scanChunks(ctx, runner, bin, hosts, cfg, func(chunk []string) []string {
		return BuildNmapArgsFor(rule, cfg, chunk)
	})
}

// scanChunks 将主机列表按 ScanChunkSize 分片，MaxParallelScans 并发执行，合并结果。
func scanChunks(ctx context.Context, runner NmapRunner, bin string, hosts []string, cfg config.DiscoveryConfig, buildArgs func([]string) []string) ([]ScanResult, error) {
	chunkSize := cfg.ScanChunkSize
	if chunkSize <= 0 {
		chunkSize = 128
	}
	parallel := cfg.MaxParallelScans
	if parallel <= 0 {
		parallel = 4
	}
	var chunks [][]string
	for i := 0; i < len(hosts); i += chunkSize {
		end := i + chunkSize
		if end > len(hosts) {
			end = len(hosts)
		}
		chunks = append(chunks, hosts[i:end])
	}

	sem := make(chan struct{}, parallel)
	results := make([][]ScanResult, len(chunks))
	var failed atomic.Int32
	var wg sync.WaitGroup
	for i, chunk := range chunks {
		wg.Add(1)
		go func(idx int, chunk []string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			scanCtx, cancel := scanContext(ctx, cfg)
			defer cancel()
			out, err := runner.Run(scanCtx, bin, buildArgs(chunk)...)
			if err != nil {
				log.Printf("discovery: chunk %d scan failed: %v", idx, err)
				failed.Add(1)
				return
			}
			rs, err := ParseNmapXML(out)
			if err != nil {
				log.Printf("discovery: chunk %d parse failed: %v", idx, err)
				failed.Add(1)
				return
			}
			results[idx] = rs
		}(i, chunk)
	}
	wg.Wait()

	if failed.Load() == int32(len(chunks)) {
		return nil, fmt.Errorf("all %d scan chunks failed", len(chunks))
	}
	var merged []ScanResult
	for _, rs := range results {
		merged = append(merged, rs...)
	}
	return merged, nil
}

// expandTargets 将规则目标（IP/CIDR 混合）展开为具体 IP 列表，受 maxHosts 上限约束。
func expandTargets(targets string, maxHosts int) ([]string, error) {
	if maxHosts <= 0 {
		maxHosts = 1024
	}
	var ips []string
	seen := map[string]bool{}
	add := func(ip string) {
		if seen[ip] {
			return
		}
		seen[ip] = true
		ips = append(ips, ip)
	}
	for _, t := range splitTargets(targets) {
		if strings.Contains(t, "/") {
			ip, ipnet, err := net.ParseCIDR(t)
			if err != nil {
				return nil, fmt.Errorf("invalid CIDR %s: %w", t, err)
			}
			for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); incIP(ip) {
				add(ip.String())
				if len(ips) >= maxHosts {
					return nil, fmt.Errorf("targets exceed max hosts (%d): %s", maxHosts, t)
				}
			}
		} else {
			ip := net.ParseIP(strings.TrimSpace(t))
			if ip == nil {
				return nil, fmt.Errorf("invalid target: %s", t)
			}
			add(ip.String())
		}
	}
	return ips, nil
}

// incIP 将 IPv4 地址字节 +1（用于 CIDR 遍历）。
func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
