# Baley tokenless MCP migration handoff

Use this handoff in every Baley-using project after the host deploys the
tokenless local MCP binary.

```text
Baley MCP uses one tokenless loopback Gateway per signed-in OS user. Do not put
a Baley token in Codex config, a project environment file, a URL, or an
Authorization header.

Host setup:
1. Install the host-provided loopback Gateway (on macOS run
   ./scripts/install-baley-mcp-macos.sh).
2. The per-user service starts `baley-mcp serve-http`; Codex connects only to
   `http://127.0.0.1:8090/mcp`. The device-bound secret remains in the OS
   Keychain / Credential Manager and is not placed in Codex configuration.
3. Fully restart Codex Desktop or begin a new Codex CLI session.

For this project:
1. Keep its baley.yaml Workspace binding unchanged.
2. Start a new Codex thread and make a read-only Baley request for that
   Workspace.
3. If the response returns `workspace_login_required`, open its loopback
   `loginUrl`, sign in with an active Workspace member, click `Connect local
   Gateway`, and retry the same request. Baley derives
   MCP scopes from that member's Workspace role; there is no separate Workspace
   connection decision.
4. If it is forbidden or not found, treat that as the genuine Workspace,
   membership, or gateway-revocation boundary. Do not bypass it with direct
   HTTP, a database, or a token.
5. Use baley_mcp_diagnostics or `baley-mcp diagnose` for redacted diagnosis.

Task confirmation, Gate changes, Gate Task pass, and Gate passage remain
human-only even though the MCP connection is automatic.
```
