package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

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
