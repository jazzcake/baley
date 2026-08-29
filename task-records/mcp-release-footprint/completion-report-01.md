# Completion report — MCP release footprint

- Record ID: `16e146c2-d92a-4f9f-9b56-2412b34e8919`
- Task: #152 MCP 릴리스 바이너리 경량화 및 메모리 프로파일링
- Completion run: `91c7e49a-47e7-4851-90ff-105e73956da5`
- Implementation commit: `b269f50513051ebf017b27169b80fd09a84686fb`

## Delivered

- Windows and macOS release installers build with `-trimpath -ldflags "-s -w"`.
- Each install uses an immutable, clean-Git-revision binary path. This avoids
  overwriting an executable held by an active stdio session.
- Registration uses Codex's same-name replacement operation directly, retaining
  the prior registration if replacement fails. No gateway token or Authorization
  header is configured.
- The Windows read-only diagnostic reports artifact size and each running
  `baley-mcp` process, plus total and average working-set/private memory.

## Measured result

Controlled identical-source build comparison:

| Artifact | Size |
| --- | ---: |
| normal Go build | 13.25 MB |
| stripped release build | 9.28 MB |
| reduction | 3.97 MB (30.0%) |

After installation of revision `b269f5051305`, the local diagnostic reported
12 running stdio sessions, 25.65 MB average working set and 50.34 MB average
private memory. The session count is intentionally one process per Codex
session; the diagnostic makes that cost observable without sharing a listener
or weakening local isolation.

## Verification

- Stripped binary `diagnose` command passed.
- Windows installer and diagnostic parsed and executed; macOS installer passed
  shell syntax validation through Git Bash.
- `codex mcp get baley` verified tokenless stdio using only a redacted server
  URL and credential-store environment entries.
- `go test ./...`, `go vet ./...`, `npm test` (87 tests), `npm run typecheck`,
  and `npm run build` all passed.
- Independent remediation review passed with no blockers.

## Boundaries preserved

Task/Gate human-only authority, Account/membership behavior, Keychain
credential storage, and the tokenless stdio transport are unchanged.
