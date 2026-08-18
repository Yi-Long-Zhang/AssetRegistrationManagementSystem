#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${APP_DIR:-/opt/asset-management}"
BACKUP_DIR="$APP_DIR/backups"
STAMP="$(date +%Y%m%d%H%M%S)"

mkdir -p "$BACKUP_DIR"

# The application backup endpoint includes SQLite, attachments, archives,
# license files, and configuration in one encrypted, checksummed archive.
response="$(curl --fail --silent --show-error \
  --request POST \
  --header "Authorization: Bearer ${BACKUP_TOKEN:?BACKUP_TOKEN is required}" \
  "http://127.0.0.1:8080/api/v1/backups")"
name="$(printf '%s' "$response" | jq -r '.name // empty')"
if [ -z "$name" ]; then
  echo "backup API did not return a backup name" >&2
  exit 1
fi

curl --fail --silent --show-error \
  --request GET \
  --header "Authorization: Bearer ${BACKUP_TOKEN}" \
  --output "$BACKUP_DIR/$name" \
  "http://127.0.0.1:8080/api/v1/backups/$(printf '%s' "$name" | jq -sRr @uri)/download"

sha256sum "$BACKUP_DIR/$name" > "$BACKUP_DIR/$name.sha256"
find "$BACKUP_DIR" -type f -name '*.abk' -mtime +"${BACKUP_KEEP_DAYS:-30}" -delete
find "$BACKUP_DIR" -type f -name '*.abk.sha256' -mtime +"${BACKUP_KEEP_DAYS:-30}" -delete

echo "backup saved to $BACKUP_DIR"
