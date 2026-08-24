#!/usr/bin/env bash
set -euo pipefail

# Installs a per-user, loopback-only Baley MCP gateway for macOS. The gateway
# calls the central Baley API only through the Tailnet Viewer /api proxy.

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
api_url="${BALEY_SERVER_URL:-https://jazzcake-home.tail87e929.ts.net/api}"
state_dir="${XDG_STATE_HOME:-$HOME/Library/Application Support}/baley-mcp"
bin_dir="${HOME}/.local/bin"
binary="${bin_dir}/baley-mcp"
token_file="${state_dir}/gateway-token"
credentials_file="${state_dir}/credentials.json"
plist="${HOME}/Library/LaunchAgents/com.baley.mcp.plist"
label="com.baley.mcp"

for command in go codex launchctl openssl; do
  command -v "$command" >/dev/null || { echo "Missing required command: $command" >&2; exit 1; }
done

case "$api_url" in
  https://jazzcake-home.tail87e929.ts.net/api|https://jazzcake-home.tail87e929.ts.net/api/)
    ;;
  *)
    echo "BALEY_SERVER_URL must be the approved Tailnet API URL" >&2
    exit 1
    ;;
esac

mkdir -p "$state_dir" "$bin_dir" "${HOME}/Library/LaunchAgents"
chmod 700 "$state_dir"
if [[ ! -f "$token_file" ]]; then
  umask 077
  openssl rand -base64 32 | tr -d '=+/\n' > "$token_file"
fi
chmod 600 "$token_file"
gateway_token="$(<"$token_file")"

go -C "${repo_root}/server" build -o "$binary" ./cmd/baley-mcp

cat > "$plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>${label}</string>
  <key>ProgramArguments</key><array><string>${binary}</string><string>serve-http</string></array>
  <key>EnvironmentVariables</key><dict>
    <key>BALEY_SERVER_URL</key><string>${api_url%/}</string>
    <key>BALEY_MCP_GATEWAY_TOKEN</key><string>${gateway_token}</string>
    <key>BALEY_MCP_CREDENTIAL_STORE</key><string>${credentials_file}</string>
    <key>BALEY_MCP_HTTP_ADDR</key><string>127.0.0.1:8091</string>
  </dict>
  <key>RunAtLoad</key><true/><key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>${state_dir}/gateway.log</string>
  <key>StandardErrorPath</key><string>${state_dir}/gateway.log</string>
</dict></plist>
PLIST
chmod 600 "$plist"

launchctl bootout "gui/${UID}" "$plist" >/dev/null 2>&1 || true
launchctl bootstrap "gui/${UID}" "$plist"
launchctl kickstart -k "gui/${UID}/${label}"
launchctl setenv BALEY_MCP_GATEWAY_TOKEN "$gateway_token"

codex mcp remove baley >/dev/null 2>&1 || true
codex mcp add baley --url http://127.0.0.1:8091/mcp --bearer-token-env-var BALEY_MCP_GATEWAY_TOKEN

echo "Baley MCP is running locally at http://127.0.0.1:8091/mcp"
echo "Restart Codex, then use /mcp to verify the Baley server."