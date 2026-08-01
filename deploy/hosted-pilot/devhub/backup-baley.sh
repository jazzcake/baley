#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
env_file="${POSTGRES_ENV_FILE:-$root/postgres.env}"
backup_root="${BALEY_BACKUP_ROOT:-/srv/devhub-postgres/backups}"
database="${BALEY_DATABASE_NAME:-baley}"
if [[ ! $database =~ ^[a-z_][a-z0-9_]{0,62}$ ]]; then
  echo "invalid database name" >&2
  exit 1
fi

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
target="$backup_root/$database-$timestamp"
mkdir --mode=0700 "$target"

compose=(docker-compose --env-file "$env_file" -f "$root/compose.yml")
"${compose[@]}" exec -T postgres pg_dumpall -U postgres --globals-only >"$target/globals.sql"
"${compose[@]}" exec -T postgres pg_dump -U baley_backup -d "$database" --format=custom --no-owner --no-privileges >"$target/database.dump"

(
  cd "$target"
  sha256sum globals.sql database.dump >SHA256SUMS
)
chmod 0600 "$target"/*
echo "$target"
