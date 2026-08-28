---
baley_record: 1
record_id: "89b0f1ac-1371-4c25-a2ec-7b11e87e2e50"
task_id: 149
task_key: "tokenless-mcp-keychain"
record_type: completion-report
run_id: "c883ef5e-ab9b-4c13-ba97-5e81cbcc8eb8"
created_at: "2026-08-28T12:20:00Z"
created_by: "codex"
supersedes: null
---

# #149 completion report

## Delivered change

Agent credentials are now strictly process-memory-only. The disk credential
store retains only routing metadata and a keychain reference, while the
keychain retains the device gateway secret; a new Desktop or CLI process must
renew with the deployed API before it can query a Workspace. Legacy plaintext
environment-file creation and related handoff guidance were removed.

## Verification

Completed successfully:

- `go test ./cmd/baley-mcp -count=1 -v`
- `go test ./...`
- `go vet ./...`
- `scripts/test-baley-mcp-env.ps1`
- `npm test` (16 files, 87 tests)
- `npm run build`

The locally deployed Windows `baley-mcp.exe` was rebuilt from this worktree and
registered with Codex as stdio. `codex mcp get baley` reports only the server
URL and credential-store environment settings, and a fresh executable reports
`keychain_backed`, an available keychain entry, and no legacy gateway setting.

## Live smoke status

A fresh registered stdio process reached the deployed Tailnet API with no
legacy gateway environment. The prior gateway registration was unavailable, so
the service correctly returned the normal signed-in Workspace connection path
instead of allowing a cached credential; completing that link would create a
new persistent gateway registration and requires explicit human approval.

## Assessment

The implementation and regression suite resolve the independent-review
blockers. Do not report Task #149 implemented yet: the final live authenticated
fresh-process Workspace read is pending the human-approved gateway enrollment;
Task confirmation remains out of scope.
