package service

import (
	"fmt"
	"strconv"
	"strings"

	"asset-registration-management-system/backend/internal/config"

	"asset-registration-management-system/backend/internal/model"
)

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
