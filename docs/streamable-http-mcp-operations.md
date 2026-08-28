---
type: operations
status: active
last_active: 2026-08-14
---

# Shared Streamable HTTP MCP

Baley runs one Streamable HTTP MCP adapter in Docker instead of starting a PowerShell and `go run` bridge for every Codex session. The adapter is local only at `http://127.0.0.1:8091/mcp`; it is not exposed through Tailscale or a public reverse proxy. A remote Mac runs its own local gateway and reaches the Baley API through the Tailnet Viewer `/api` proxy.

## Security model

Codex sends `Authorization: Bearer <gateway token>` on every MCP request. This local loopback gateway token authenticates transport access only; it is never forwarded to the Baley API. One local gateway identity keeps one persistent credential store: each Workspace Agent token and pending connection secret is AES-256-GCM encrypted with a key derived from that gateway token and the canonical API URL. The existing `workspace_connection_required → approvalUrl → retry` flow is therefore required only for a new Workspace, a revoked Agent token, or local gateway-token rotation—not for a new Codex chat or MCP transport session. The API remains the authority for Agent identity, Workspace tenancy, capability checks, revision CAS, and human approval attestations.

The gateway token is generated and synchronized by `scripts/sync-baley-mcp-http-token.ps1`. Codex supports a Streamable HTTP URL plus an environment-variable name, not a dotenv file, so the script copies it into the current and per-user `BALEY_MCP_GATEWAY_TOKEN` environment. Treat that user environment value as a local secret. The legacy `.env.baley-mcp.local` Agent token remains for the stdio rollback path only.

After the one-time Owner-approved enrollment, the API registers the local gateway
with that active Workspace member. It stores only a hash of a separate gateway
credential; the local encrypted store retains the credential. A new MCP transport
session can renew its Workspace-scoped Agent token through this registration without
another approval. Every renewal validates the gateway, its Workspace, the registered
member's active Account/membership, and the Agent binding. Member role/removal,
gateway replacement, explicit revoke, or suspected compromise immediately revoke all
derived Agent tokens and require a new approval. This does not grant human-only
Task or Gate capabilities.

## Start and register Codex

```powershell
docker compose up -d --build api viewer mcp
.\scripts\prepare-local-pilot-agent.ps1
.\scripts\sync-baley-mcp-http-token.ps1
codex mcp remove baley
codex mcp add baley --url http://127.0.0.1:8091/mcp --bearer-token-env-var BALEY_MCP_GATEWAY_TOKEN
```

Restart the Codex app, then open a new thread. The registration has no command or `go run` launcher, so new sessions connect to the same container.

## Check and troubleshoot

```powershell
docker compose ps
docker compose logs --tail 100 mcp
Invoke-WebRequest http://127.0.0.1:8091/mcp -Method Post -ContentType application/json
codex mcp get baley
```

The unauthenticated request intentionally returns `401`. If Codex gets `401`, synchronize the token and restart Codex. A forbidden or not-found tool result means the token belongs to another Workspace or lacks the required capability; do not weaken the API authorization boundary.

## Tailnet Codex clients

Each Mac runs its own loopback-only MCP gateway, gateway token, and encrypted credential store. The local gateway reaches the Baley API at `https://jazzcake-home.tail87e929.ts.net/api` through Tailscale; it does not expose or consume a remote `/mcp` endpoint.

On a Baley checkout on the Mac, run:

```bash
./scripts/install-baley-mcp-macos.sh
```

The script builds `baley-mcp`, installs a per-user launchd service on `127.0.0.1:8091`, creates a per-Mac gateway token and credential store, and registers Codex against `http://127.0.0.1:8091/mcp`. It never asks for or copies the token from the Baley host.

### Gateway token setup on macOS

The installer stores the Mac-local secret at:

```text
~/Library/Application Support/baley-mcp/gateway-token
```

Do not print, paste, commit, or transfer this value. The installer applies it in two separate places:

- **Codex Desktop:** `launchctl setenv` receives the token. Fully quit and reopen Codex Desktop after the installer finishes.
- **Codex CLI:** source the generated, owner-readable-only shell file in each terminal before launching Codex:

```bash
source "$HOME/Library/Application Support/baley-mcp/codex-cli-env.sh"
codex
```

Check presence without printing the secret:

```bash
launchctl getenv BALEY_MCP_GATEWAY_TOKEN >/dev/null && echo "gateway token is available to Desktop apps"
test -r "$HOME/Library/Application Support/baley-mcp/codex-cli-env.sh" && echo "CLI environment file exists"
codex mcp get baley
```

Do not expose port `8091` or copy one Mac's gateway token to another device.
Docker restart recovery is automatic (`restart: unless-stopped`). Verify with `docker compose restart mcp` and a new Codex thread/tool listing.

## Migration and rollback

`scripts/run-baley-mcp.ps1` remains a development-only stdio compatibility entry point during migration. It is no longer the default Codex registration. To roll back temporarily:

```powershell
codex mcp remove baley
codex mcp add baley -- powershell.exe -NoProfile -ExecutionPolicy Bypass -File D:\Project_AI\baley\scripts\run-baley-mcp.ps1
```

Restart Codex after either change. Do not run both registrations under the same `baley` name.
