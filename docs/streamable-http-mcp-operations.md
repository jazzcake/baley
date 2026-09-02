---
type: operations
status: active
last_active: 2026-08-28
---

# Tokenless local MCP operations

On Windows, Baley uses one tokenless loopback MCP Gateway per signed-in user for
Codex Desktop and CLI. Codex connects to `http://127.0.0.1:8090/mcp`; the
Gateway alone receives the configured Tailnet HTTPS API URL and local
credential-store path. `BALEY_MCP_GATEWAY_TOKEN` is not placed in
`config.toml`, shell profiles, Codex Desktop environment, or an Authorization
header.

## Security model

The local JSON store has only server routing metadata and an opaque keychain
reference. Gateway secrets and pending connection secrets are stored in the OS
credential manager: macOS Keychain, Windows Credential Manager, or Linux Secret
Service. A Workspace Agent credential exists only in the running MCP process;
a fresh process renews it from the registered gateway. The Keychain item is
device-bound and is never printed by diagnostics.

The local Gateway may connect to the remote Baley API only with HTTPS. It binds
only to loopback, is not a Tailscale or public endpoint, and reads device
credentials only through the OS credential manager.

Loopback is a same-machine trust boundary, not an OS-user authentication
boundary. Do not run the Baley Gateway on a shared or untrusted desktop; Baley
does not provide a command-based per-session fallback. No external network peer
can reach the loopback endpoint.

The first device is linked through an MCP-visible loopback `loginUrl` that
verifies the pending local request before redirecting to Baley. An active
member signs in and explicitly clicks `Connect local Gateway`; the browser gets
a two-minute one-time code, and only the local gateway holding the matching
pending connection secret may redeem it. The same registered device is then automatically enrolled in any
Workspace where that Account has active membership; there is no separate
Workspace connection decision. Owner/Operator roles receive ordinary Agent
operation scopes, while Viewer/Approver roles receive read-only Agent scope.
Logout, membership removal, gateway replace,
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

The Windows installer builds a stripped (`-trimpath -ldflags "-s -w"`) release
binary in a Git-revisioned path, registers a per-user logon task that owns one
loopback Gateway, and changes Codex atomically to its local HTTP endpoint. The
macOS installer provides the same single-Gateway model through a per-user
LaunchAgent and macOS Keychain. No firewall rule is required or created on
either platform.

The equivalent CLI registration is:

```bash
codex mcp remove baley
codex mcp add baley --url http://127.0.0.1:8090/mcp
```

Restart Codex Desktop fully after changing its registration. Only a first device
or a device invalidated by logout, membership removal, replacement, or revoke
uses the signed-in gateway login flow.

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
rejects its next renewal. After the window, link again by signing in to Baley
instead of retaining legacy material.

Earlier plaintext stores are migrated directly into the OS keychain but are
never copied back to disk for rollback; preserving a plaintext backup would
violate the tokenless storage boundary. Their safe rollback is a fresh member
login link, not revival of the old disk secret.

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
