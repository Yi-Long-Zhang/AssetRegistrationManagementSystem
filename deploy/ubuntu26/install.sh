#!/usr/bin/env bash
set -euo pipefail

APP_NAME="asset-management"
APP_USER="assetmgmt"
APP_DIR="${APP_DIR:-/opt/asset-management}"
PACKAGE_DIR="$(cd "$(dirname "$0")" && pwd)"
ENV_SOURCE="$PACKAGE_DIR/config/asset-management.env"
ENV_TARGET="$APP_DIR/.env"

if [ "$(id -u)" -ne 0 ]; then
  echo "请使用 root 或 sudo 执行: sudo bash install.sh"
  exit 1
fi

if [ ! -f "$ENV_SOURCE" ] && [ ! -f "$ENV_TARGET" ]; then
  echo "未找到 $ENV_SOURCE，且 $ENV_TARGET 不存在。"
  echo "首次安装请先复制 config/asset-management.env.example 为 config/asset-management.env 并修改密钥和管理员密码。"
  exit 1
fi

apt-get update
apt-get install -y nginx ca-certificates tar rsync libreoffice-writer fonts-noto-cjk

if ! id "$APP_USER" >/dev/null 2>&1; then
  useradd --system --home "$APP_DIR" --shell /usr/sbin/nologin "$APP_USER"
fi

install -d -m 0755 "$APP_DIR"
install -d -m 0755 "$APP_DIR/backend"
install -d -m 0755 "$APP_DIR/frontend"
install -d -m 0750 "$APP_DIR/data"
install -d -m 0750 "$APP_DIR/data/attachments"
install -d -m 0750 "$APP_DIR/data/ticket-archives"
install -d -m 0750 "$APP_DIR/backups"
install -d -m 0755 "$APP_DIR/scripts"
install -d -m 0755 "$APP_DIR/templates"

if systemctl is-active --quiet "$APP_NAME"; then
  systemctl stop "$APP_NAME"
fi

install -m 0755 "$PACKAGE_DIR/backend/asset-management-server" "$APP_DIR/backend/asset-management-server"
rsync -a --delete "$PACKAGE_DIR/frontend/" "$APP_DIR/frontend/"
if [ -d "$PACKAGE_DIR/templates" ]; then
  rsync -a --delete "$PACKAGE_DIR/templates/" "$APP_DIR/templates/"
fi
if [ -f "$ENV_SOURCE" ]; then
  install -m 0640 "$ENV_SOURCE" "$ENV_TARGET"
fi
install -m 0755 "$PACKAGE_DIR/scripts/backup.sh" "$APP_DIR/scripts/backup.sh"

chown -R "$APP_USER:$APP_USER" "$APP_DIR"
chmod 0750 "$APP_DIR/data" "$APP_DIR/data/attachments" "$APP_DIR/data/ticket-archives" "$APP_DIR/backups"
chmod 0640 "$ENV_TARGET"

install -m 0644 "$PACKAGE_DIR/systemd/asset-management.service" "/etc/systemd/system/$APP_NAME.service"
install -m 0644 "$PACKAGE_DIR/nginx/asset-management.conf" "/etc/nginx/sites-available/$APP_NAME.conf"
ln -sfn "/etc/nginx/sites-available/$APP_NAME.conf" "/etc/nginx/sites-enabled/$APP_NAME.conf"
rm -f /etc/nginx/sites-enabled/default

nginx -t
systemctl daemon-reload
systemctl enable "$APP_NAME"
systemctl restart "$APP_NAME"
systemctl reload nginx

echo "部署完成。"
echo "后端状态: systemctl status $APP_NAME"
echo "访问地址: http://服务器IP/"
