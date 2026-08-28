---
type: operations
status: active
last_active: 2026-08-28
---

# Tokenless local MCP operations

Baley uses a tokenless stdio MCP process for Codex Desktop and CLI. Codex starts
`baley-mcp` directly and supplies only the approved Tailnet HTTPS API URL and a
local credential-store path. `BALEY_MCP_GATEWAY_TOKEN` is not placed in
`config.toml`, shell profiles, Codex Desktop environment, or an Authorization
header.

## Security model

The local JSON store has only server routing metadata and an opaque keychain
reference. Gateway secrets, pending connection secrets, and cached Workspace
Agent credentials are stored in the OS credential manager: macOS Keychain,
Windows Credential Manager, or Linux Secret Service. The Keychain item is
device-bound and is never printed by diagnostics.

The stdio process may connect to the remote Baley API only with HTTPS. Optional
legacy Streamable HTTP compatibility must listen on a loopback address and is
not a Tailscale or public endpoint. It is not the supported Codex registration.

After the one-time Owner-approved enrollment, #142 registers the local gateway
for an active Workspace member. A fresh MCP process renews an Operator-only
credential from that registration. Logout, membership removal, gateway replace,
suspected compromise, or server-side revoke invalidates the gateway and derived
credentials. No path grants Task confirmation, Gate condition changes, Gate
Task pass, or Gate pass authority.

## Codex registration

On a Mac checkout, run:

```bash
./scripts/install-baley-mcp-macos.sh
```

On Windows, run from the repository checkout:

```powershell
.\scripts\install-baley-mcp-windows.ps1
```

The equivalent CLI registration is:

```bash
codex mcp remove baley
codex mcp add baley \
  --env BALEY_SERVER_URL=https://jazzcake-home.tail87e929.ts.net/api \
  --env "BALEY_MCP_CREDENTIAL_STORE=$HOME/Library/Application Support/baley-mcp/credentials.json" \
  -- /path/to/baley-mcp
```

Restart Codex Desktop fully after changing its registration. The first request
for an unregistered Workspace returns the normal signed-in gateway link; retry the
same request after approval.

## Workspace discovery payload

Start an unfamiliar Workspace with `baley_workspace_context`. It is a revisioned
summary of non-completed Phases and per-Lane status counts; it deliberately omits
Task identities, descriptions, graph edges, evidence, and completed Phases. Treat
`workspace.revision` as the snapshot marker: refresh the compact summary when it
changes, rather than assuming a delta history is embedded in the response.

Use `baley_phase_tasks` only after selecting one returned Phase ID. Its public-ID
cursor is scoped by that explicit Phase and the page size is 1–100 (50 by default).
Use `baley_workspace_graph` only for callers that explicitly need the full,
compatibility-preserved Viewer-style projection.

## Migration, rollback, and diagnostics

A legacy encrypted credential store can be opened once with its existing local
gateway token, then rewritten as a Keychain-backed store. Run the migration in
an interactive shell only; do not add that variable to Codex configuration:

```bash
BALEY_MCP_GATEWAY_TOKEN='existing-local-value' baley-mcp migrate-legacy
```

The migration clears the cached Agent token and revalidates every registered
gateway with the server before it reports success. It never writes a decrypted
secret to disk. A 15-minute encrypted rollback copy is retained solely for a
failed local migration; use `baley-mcp rollback-legacy` with the existing local
value during that window. The rollback restores only the former local file and
does not restore a revoked gateway or removed membership: the server still
rejects its next renewal. After the window, enroll again while signed in to Baley
instead of retaining legacy material.

Earlier plaintext stores are migrated directly into the OS keychain but are
never copied back to disk for rollback; preserving a plaintext backup would
violate the tokenless storage boundary. Their safe rollback is a fresh Owner
approval, not revival of the old disk secret.

Use `baley_mcp_diagnostics` for redacted state only:

- `keychain_backed` means the disk file has no credential material and the OS
  keychain entry can be read.
- `legacy_migration_required` means the old local token is needed once to move
  its encrypted store.
- `not_created` is normal before the first Workspace connection.

If Keychain access is unavailable, stop and fix the OS credential-manager
permission. Baley must not fall back to a plaintext file or a Codex token
environment variable. `baley-mcp diagnose` provides the same redacted local
state before Codex is configured.
