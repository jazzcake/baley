#!/usr/bin/env bash
set -euo pipefail

# Installs one per-user tokenless loopback MCP Gateway. The binary contacts the
# Tailnet HTTPS API directly; device and Workspace credentials are placed in
# the macOS Keychain, never in Codex configuration or an environment token.

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
api_url="${BALEY_SERVER_URL:-https://jazzcake-home.tail87e929.ts.net/api}"
state_dir="${XDG_STATE_HOME:-$HOME/Library/Application Support}/baley-mcp"
bin_dir="${HOME}/.local/bin"
if [[ -n "$(git -C "${repo_root}" status --porcelain)" ]]; then
  echo "Commit or stash Baley MCP source changes before creating a release install" >&2
  exit 1
fi
release_id="$(git -C "${repo_root}" rev-parse --short=12 HEAD)"
if [ -z "${release_id}" ]; then
  echo "Unable to determine the Baley MCP release ID" >&2
  exit 1
fi
binary="${bin_dir}/baley-mcp-releases/${release_id}/baley-mcp"
credentials_file="${state_dir}/credentials.json"
legacy_plist="${HOME}/Library/LaunchAgents/com.baley.mcp.plist"
launch_agents_dir="${HOME}/Library/LaunchAgents"
gateway_plist="${launch_agents_dir}/com.baley.mcp.plist"
loopback_url="http://127.0.0.1:8090/mcp"

for command in go codex launchctl curl plutil; do
  command -v "$command" >/dev/null || { echo "Missing required command: $command" >&2; exit 1; }
done

case "$api_url" in
  https://jazzcake-home.tail87e929.ts.net/api|https://jazzcake-home.tail87e929.ts.net/api/)
    ;;
  *)
    echo "BALEY_SERVER_URL must be the approved Tailnet HTTPS API URL" >&2
    exit 1
    ;;
esac

mkdir -p "$state_dir" "$(dirname "$binary")"
chmod 700 "$state_dir"
# Keep the source path and debug tables out of the release artifact without
# changing its keychain or tokenless transport behavior.
if [ ! -f "$binary" ]; then
  go -C "${repo_root}/server" build -trimpath -ldflags="-s -w" -o "$binary" ./cmd/baley-mcp
fi

# One LaunchAgent owns the callback listener and MCP endpoint for every Codex
# Desktop/CLI session. Both endpoints bind only to 127.0.0.1.
mkdir -p "$launch_agents_dir"
launchctl bootout "gui/${UID}" "$legacy_plist" >/dev/null 2>&1 || true
cat >"${gateway_plist}.tmp" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>com.baley.mcp</string>
  <key>ProgramArguments</key><array><string>${binary}</string><string>serve-http</string></array>
  <key>EnvironmentVariables</key><dict>
    <key>BALEY_SERVER_URL</key><string>${api_url%/}</string>
    <key>BALEY_MCP_CREDENTIAL_STORE</key><string>${credentials_file}</string>
    <key>BALEY_MCP_HTTP_ADDR</key><string>127.0.0.1:8090</string>
  </dict>
  <key>RunAtLoad</key><true/><key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>${state_dir}/gateway.log</string>
  <key>StandardErrorPath</key><string>${state_dir}/gateway-error.log</string>
</dict></plist>
PLIST
plutil -lint "${gateway_plist}.tmp" >/dev/null
mv "${gateway_plist}.tmp" "$gateway_plist"
chmod 600 "$gateway_plist"
launchctl bootstrap "gui/${UID}" "$gateway_plist"
launchctl kickstart -k "gui/${UID}/com.baley.mcp"

ready=0
for _ in $(seq 1 30); do
  if curl --silent --show-error --fail --max-time 2 \
    -H 'Accept: application/json, text/event-stream' -H 'Content-Type: application/json' \
    --data '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"baley-installer","version":"1"}}}' \
    "$loopback_url" | grep -q 'serverInfo'; then ready=1; break; fi
  sleep 0.25
done
if [[ "$ready" != 1 ]]; then
  echo "Baley loopback Gateway did not become ready" >&2
  exit 1
fi

# `codex mcp add` replaces the named registration atomically. No secret or
# Authorization header is written to Codex configuration.
codex mcp add baley --url "$loopback_url"

echo "Baley MCP is registered through one tokenless loopback Gateway."
echo "Codex Desktop: fully quit and reopen it. Codex CLI: start a new session."
echo "For a first device, open the local login URL, sign in, and click 'Connect local Gateway'."
echo "Run 'codex mcp get baley' or the baley_mcp_diagnostics tool for redacted diagnostics."
