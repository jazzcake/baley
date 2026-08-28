# Baley tokenless MCP migration handoff

Use this handoff in every Baley-using project after the host deploys the
tokenless local MCP binary.

```text
Baley MCP uses a local tokenless stdio process. Do not put a Baley token in
Codex config, a project environment file, a URL, or an Authorization header.

Host setup:
1. Install the host-provided tokenless registration (on macOS run
   ./scripts/install-baley-mcp-macos.sh).
2. Codex starts baley-mcp directly with BALEY_SERVER_URL and
   BALEY_MCP_CREDENTIAL_STORE only. The device-bound secret remains in the OS
   Keychain / Credential Manager.
3. Fully restart Codex Desktop or begin a new Codex CLI session.

For this project:
1. Keep its baley.yaml Workspace binding unchanged.
2. Start a new Codex thread and make a read-only Baley request for that
   Workspace.
3. If the response requests Workspace connection approval, have the signed-in
   Workspace Owner approve the normal URL and retry the same request.
4. If it is forbidden or not found, treat that as the genuine Workspace,
   membership, or gateway-revocation boundary. Do not bypass it with direct
   HTTP, a database, or a token.
5. Use baley_mcp_diagnostics or `baley-mcp diagnose` for redacted diagnosis.

Task confirmation, Gate changes, Gate Task pass, and Gate passage remain
human-only even though the MCP connection is automatic.
```
