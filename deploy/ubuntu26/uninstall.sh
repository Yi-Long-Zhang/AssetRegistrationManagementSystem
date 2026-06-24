#!/usr/bin/env bash
set -euo pipefail

APP_NAME="asset-management"

if [ "$(id -u)" -ne 0 ]; then
  echo "请使用 root 或 sudo 执行: sudo bash uninstall.sh"
  exit 1
fi

systemctl stop "$APP_NAME" 2>/dev/null || true
systemctl disable "$APP_NAME" 2>/dev/null || true
rm -f "/etc/systemd/system/$APP_NAME.service"
rm -f "/etc/nginx/sites-enabled/$APP_NAME.conf"
rm -f "/etc/nginx/sites-available/$APP_NAME.conf"
systemctl daemon-reload
nginx -t && systemctl reload nginx

echo "服务和 Nginx 配置已移除。/opt/asset-management 数据目录未删除。"
