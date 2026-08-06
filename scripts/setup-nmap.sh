#!/usr/bin/env bash
# 安装 nmap（Linux/macOS 平台）
# Linux 官方无便携 zip 包，这里通过系统包管理器安装（安装到 PATH，代码会自动探测）
# 用法: bash scripts/setup-nmap.sh
set -euo pipefail

if command -v nmap >/dev/null 2>&1; then
    echo "nmap already available: $(command -v nmap)"
    nmap --version | head -1
    exit 0
fi

case "$(uname)" in
    Darwin)
        if command -v brew >/dev/null 2>&1; then
            brew install nmap
        else
            echo "请先安装 Homebrew（https://brew.sh）或从 https://nmap.org/download 下载 macOS 安装包" >&2
            exit 1
        fi
        ;;
    *)
        if command -v apt-get >/dev/null 2>&1; then
            sudo apt-get update && sudo apt-get install -y nmap
        elif command -v dnf >/dev/null 2>&1; then
            sudo dnf install -y nmap
        elif command -v yum >/dev/null 2>&1; then
            sudo yum install -y nmap
        elif command -v zypper >/dev/null 2>&1; then
            sudo zypper install -y nmap
        else
            echo "未识别的包管理器，请手动安装 nmap（https://nmap.org/download）" >&2
            exit 1
        fi
        ;;
esac

nmap --version | head -1
echo "nmap installed: $(command -v nmap)"
