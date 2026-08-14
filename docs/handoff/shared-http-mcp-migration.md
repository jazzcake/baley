# Baley shared HTTP MCP migration — handoff prompt

Use this prompt in every Baley-using project after the Baley host has deployed
the shared Streamable HTTP MCP adapter.

```text
Baley MCP has moved from a per-session stdio launcher to the shared local
Streamable HTTP endpoint.

Do not add or run D:\Project_AI\baley\scripts\run-baley-mcp.ps1 in this
project's Codex configuration. It is rollback-only development compatibility.

Prerequisites on the host:
1. Baley Docker services include a healthy `mcp` container.
2. Codex is registered as:
   codex mcp add baley --url http://127.0.0.1:8091/mcp --bearer-token-env-var BALEY_AGENT_TOKEN
3. BALEY_AGENT_TOKEN is synchronized from Baley's local agent environment and
   Codex has been restarted after synchronization.

For this project:
1. Keep its `baley.yaml` Workspace binding unchanged.
2. Start a new Codex thread and call a read-only Baley tool for that Workspace.
3. If the response is unauthenticated, do not put a token in a URL, project
   config, prompt, or log. Ask the Baley host operator to run
   scripts\sync-baley-mcp-http-token.ps1 and restart Codex.
4. If the response is forbidden or not found, treat it as a genuine Workspace
   token/capability boundary. Use the Baley connection/approval workflow; do
   not bypass it with direct HTTP or database access.
5. Do not create a per-project MCP launcher, worktree-specific token file, or
   `go run ./cmd/baley-mcp` process. All Codex sessions share the Docker MCP
   adapter.

Refer to D:\Project_AI\baley\docs\streamable-http-mcp-operations.md for
operations, diagnosis, and the temporary stdio rollback procedure.
```
