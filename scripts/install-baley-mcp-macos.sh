#!/usr/bin/env bash
set -euo pipefail

# Installs Baley as a tokenless stdio MCP server. The binary contacts the
# Tailnet HTTPS API directly; device and Workspace credentials are placed in
# the macOS Keychain, never in Codex configuration or an environment token.

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
api_url="${BALEY_SERVER_URL:-https://jazzcake-home.tail87e929.ts.net/api}"
state_dir="${XDG_STATE_HOME:-$HOME/Library/Application Support}/baley-mcp"
bin_dir="${HOME}/.local/bin"
binary="${bin_dir}/baley-mcp"
credentials_file="${state_dir}/credentials.json"
legacy_plist="${HOME}/Library/LaunchAgents/com.baley.mcp.plist"

for command in go codex launchctl; do
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

mkdir -p "$state_dir" "$bin_dir"
chmod 700 "$state_dir"
go -C "${repo_root}/server" build -o "$binary" ./cmd/baley-mcp

# Stop the retired HTTP gateway if it exists. Its files are deliberately left
# in place so a one-time legacy-store migration can still be diagnosed; the
# installer neither prints nor exports its former gateway token.
if [[ -f "$legacy_plist" ]]; then
  launchctl bootout "gui/${UID}" "$legacy_plist" >/dev/null 2>&1 || true
fi

codex mcp remove baley >/dev/null 2>&1 || true
codex mcp add baley \
  --env "BALEY_SERVER_URL=${api_url%/}" \
  --env "BALEY_MCP_CREDENTIAL_STORE=${credentials_file}" \
  -- "$binary"

echo "Baley MCP is registered as tokenless stdio."
echo "Codex Desktop: fully quit and reopen it. Codex CLI: start a new session."
echo "The first Workspace request may ask its Owner for approval; later sessions resume through the macOS Keychain."
echo "Run 'codex mcp get baley' or the baley_mcp_diagnostics tool for redacted diagnostics."
