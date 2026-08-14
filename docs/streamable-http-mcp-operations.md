---
type: operations
status: active
last_active: 2026-08-14
---

# Shared Streamable HTTP MCP

Baley runs one Streamable HTTP MCP adapter in Docker instead of starting a PowerShell and `go run` bridge for every Codex session. The adapter is local only at `http://127.0.0.1:8091/mcp`; it is not exposed through Tailscale or a public reverse proxy.

## Security model

Codex sends `Authorization: Bearer <token>` on every MCP request. The adapter validates that token against the Baley API before accepting the request, then forwards it only to the API for the matching tool call. It keeps no Workspace credential store and never accepts a token in a URL, query string, or request log. The API remains the authority for Agent identity, Workspace tenancy, capability checks, revision CAS, and human approval attestations.

The local token continues to live in `.env.baley-mcp.local`. Codex supports a Streamable HTTP URL plus an environment-variable name, not a dotenv file, so the setup script copies the token into the current and per-user `BALEY_AGENT_TOKEN` environment. Treat that user environment value as a local secret and rerun the script after issuing a replacement token.

## Start and register Codex

```powershell
docker compose up -d --build api viewer mcp
.\scripts\prepare-local-pilot-agent.ps1
.\scripts\sync-baley-mcp-http-token.ps1
codex mcp remove baley
codex mcp add baley --url http://127.0.0.1:8091/mcp --bearer-token-env-var BALEY_AGENT_TOKEN
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

Docker restart recovery is automatic (`restart: unless-stopped`). Verify with `docker compose restart mcp` and a new Codex thread/tool listing.

## Migration and rollback

`scripts/run-baley-mcp.ps1` remains a development-only stdio compatibility entry point during migration. It is no longer the default Codex registration. To roll back temporarily:

```powershell
codex mcp remove baley
codex mcp add baley -- powershell.exe -NoProfile -ExecutionPolicy Bypass -File D:\Project_AI\baley\scripts\run-baley-mcp.ps1
```

Restart Codex after either change. Do not run both registrations under the same `baley` name.
