package service

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"asset-registration-management-system/backend/internal/config"

	"asset-registration-management-system/backend/internal/model"
)

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
