# Production Readiness

## Health checks

- `/livez` only verifies that the HTTP process is alive.
- `/readyz` verifies the database, writable storage directories, LibreOffice,
  nmap, and the persistent task processor. A non-200 response must remove the
  instance from service.
- `/metrics` exposes request and task counters for a Prometheus-compatible
  collector. Do not expose it publicly without network controls.

## Release procedure

1. Put the service in maintenance mode or stop writes.
2. Create and verify a complete encrypted backup from the administrator
   console. Record its SHA-256 value and content manifest.
3. Run the migration compatibility check against a copy of the production
   database.
4. Deploy the backend and frontend artifacts, then restart systemd.
5. Verify `/livez`, `/readyz`, login, asset list, ticket list, and backup
   visibility.
6. Keep the previous artifact and database backup until business verification
   is complete. Roll back the artifact and restore the backup if migration or
   health verification fails.

## Scheduled backup

`deploy/ubuntu26/scripts/backup.sh` creates the same complete `.abk` backup
format used by the API. Run it from a protected systemd timer or cron entry
with `BACKUP_TOKEN` supplied through a root-readable environment file. Store
the archive and checksum on a separate host and periodically verify a restore
in an empty environment.

## Incident checks

- Revoke all sessions after a suspected credential leak.
- Rotate the active JWT key after recording the previous key identifier needed
  for a controlled overlap window.
- Inspect the administrator audit log for credential, license, backup, and
  session actions.
- Use the task operations center to retry failed idempotent tasks and
  acknowledge the alert after confirming the result.
