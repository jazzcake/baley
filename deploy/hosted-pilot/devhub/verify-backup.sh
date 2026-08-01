#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 /absolute/path/to/backup-directory" >&2
  exit 1
fi

root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
env_file="${POSTGRES_ENV_FILE:-$root/postgres.env}"
backup_dir="$(realpath "$1")"
for required in SHA256SUMS database.dump; do
  if [[ ! -f "$backup_dir/$required" ]]; then
    echo "backup is missing $required" >&2
    exit 1
  fi
done
(
  cd "$backup_dir"
  sha256sum --check SHA256SUMS
)

verify_db="baley_restore_verify_$(date -u +%Y%m%d%H%M%S)_${BASHPID}"
compose=(docker-compose --env-file "$env_file" -f "$root/compose.yml")
created_verify_db=0
cleanup() {
  if [[ $created_verify_db -eq 1 ]]; then
    "${compose[@]}" exec -T postgres dropdb --if-exists --force -U postgres "$verify_db" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

"${compose[@]}" exec -T postgres createdb -U postgres "$verify_db"
created_verify_db=1
"${compose[@]}" exec -T postgres pg_restore -U postgres -d "$verify_db" --no-owner --no-privileges --exit-on-error --single-transaction <"$backup_dir/database.dump"
workspace_count="$("${compose[@]}" exec -T postgres psql -U postgres -d "$verify_db" -Atc 'SELECT count(*) FROM workspaces')"
if [[ ! $workspace_count =~ ^[0-9]+$ ]]; then
  echo "restored database validation failed" >&2
  exit 1
fi
echo "isolated restore verified: workspaces=$workspace_count"
