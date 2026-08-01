#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
env_file="${1:-$root/postgres.env}"
if [[ ! -f "$env_file" ]]; then
  echo "environment file not found: $env_file" >&2
  exit 1
fi

set -a
source "$env_file"
set +a

: "${POSTGRES_BIND_ADDRESS:?POSTGRES_BIND_ADDRESS is required}"
: "${POSTGRES_ADMIN_PASSWORD_FILE:?POSTGRES_ADMIN_PASSWORD_FILE is required}"

tailnet_ip="$(tailscale ip -4 | head -n 1)"
if [[ -z "$tailnet_ip" || "$POSTGRES_BIND_ADDRESS" != "$tailnet_ip" ]]; then
  echo "POSTGRES_BIND_ADDRESS must equal this host's Tailscale IPv4 address" >&2
  exit 1
fi
if [[ ! -f "$POSTGRES_ADMIN_PASSWORD_FILE" ]]; then
  echo "admin password file not found" >&2
  exit 1
fi
if [[ $(stat -c '%a' "$POSTGRES_ADMIN_PASSWORD_FILE") != "600" ]]; then
  echo "admin password file mode must be 600" >&2
  exit 1
fi

docker-compose --env-file "$env_file" -f "$root/compose.yml" config >/dev/null
echo "devhub PostgreSQL configuration is valid"
