#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${APP_DIR:-/opt/asset-management}"
BACKUP_DIR="$APP_DIR/backups"
STAMP="$(date +%Y%m%d%H%M%S)"

mkdir -p "$BACKUP_DIR"

if [ -f "$APP_DIR/data/assets.db" ]; then
  cp "$APP_DIR/data/assets.db" "$BACKUP_DIR/assets.db.$STAMP"
fi

if [ -d "$APP_DIR/data/attachments" ]; then
  tar -czf "$BACKUP_DIR/attachments.$STAMP.tar.gz" -C "$APP_DIR/data" attachments
fi

echo "backup saved to $BACKUP_DIR"
