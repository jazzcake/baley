# Independent review — MCP release footprint

- Record ID: `6926c62c-2ba7-4864-8eb6-c3f8b1696c90`
- Task: #152 MCP 릴리스 바이너리 경량화 및 메모리 프로파일링
- Review run: `4163d51e-0785-4858-bdd7-106dfff8d3a6`

## Scope

Review the release build flags, Windows/macOS installers, diagnostic script,
and tokenless transport/configuration boundary.

## Initial findings

1. A HEAD-only release directory could reuse a stale binary after a dirty-tree
   source edit.
2. Removing the existing Codex MCP registration before adding the replacement
   could leave the next session without Baley if the add failed.
3. Windows accepted an arbitrary HTTPS endpoint while the other installer and
   operations documentation restricted the approved endpoint.
4. The diagnostic needed aggregate totals and averages to make snapshots useful
   for comparing idle and active session footprint.

## Review response

- Release installs now require a clean Git worktree, so the immutable release
  ID always identifies committed source.
- A local CLI probe verified that `codex mcp add` replaces an existing named
  stdio registration. Installers now use that replacement directly and no
  longer remove the existing registration first.
- Windows now enforces the same approved Tailnet API endpoint as macOS.
- The read-only Windows diagnostic now reports total and average private and
  working-set memory, as well as each process.

## Verdict

Passed after remediation. No gateway token or Authorization header is stored:
the validated registration is tokenless stdio with only the server URL and
credential-store location.
