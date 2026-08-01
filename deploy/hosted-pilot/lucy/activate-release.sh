#!/usr/bin/env bash
set -euo pipefail

if [[ ${EUID} -ne 0 ]]; then
  echo "run as root" >&2
  exit 1
fi
if [[ $# -ne 1 || ! $1 =~ ^baley-[0-9A-Za-z][0-9A-Za-z._-]{0,63}-linux-amd64$ ]]; then
  echo "usage: $0 baley-VERSION-linux-amd64" >&2
  exit 1
fi

release_root="/srv/baley/releases/$1"
current_link="/srv/baley/current"
if [[ ! -d "$release_root" || ! -x "$release_root/baley-server" || ! -f "$release_root/SHA256SUMS" ]]; then
  echo "release is incomplete: $release_root" >&2
  exit 1
fi
if [[ -e "$current_link" && ! -L "$current_link" ]]; then
  echo "refusing to replace non-symlink path: $current_link" >&2
  exit 1
fi

cd "$release_root"
sha256sum --check SHA256SUMS

previous=""
if [[ -L "$current_link" ]]; then
  previous="$(readlink -f "$current_link")"
fi

restore_link() {
  if [[ -n "$previous" && -d "$previous" ]]; then
    ln -sfnT "$previous" "$current_link"
  else
    rm -f -- "$current_link"
  fi
}

restore_services() {
  restore_link
  if [[ -n "$previous" && -d "$previous" ]]; then
    systemctl restart baley.service baley-web.service || true
  else
    systemctl stop baley.service baley-web.service || true
  fi
}

ln -sfnT "$release_root" "$current_link"
if ! systemctl start baley-migrate.service; then
  restore_link
  echo "release migration failed; release link restored" >&2
  exit 1
fi
if ! systemctl restart baley.service baley-web.service; then
  restore_services
  echo "release service restart failed; previous release restored when available" >&2
  exit 1
fi

ready=0
for _ in {1..20}; do
  if curl --fail --silent --show-error http://127.0.0.1:8080/readyz >/dev/null; then
    ready=1
    break
  fi
  sleep 1
done

if [[ $ready -ne 1 ]]; then
  restore_services
  echo "release readiness failed; previous release restored when available" >&2
  exit 1
fi

curl --fail --silent --show-error http://127.0.0.1:8081/api/versionz
echo
echo "activated $release_root"
