---
baley_record: 1
record_id: "2d3e92a6-7f08-47ef-8b88-7ae410b1f6ec"
task_id: 180
task_key: "mcp-tool-catalog"
record_type: independent-agent-review
run_id: null
created_at: "2026-09-06T14:51:00Z"
created_by: "codex-pm"
registration_state: pending
supersedes: null
---

# #180 independent review: compact MCP catalog

## Verdict

Pass with zero unresolved blocking findings.

## Review performed

- Inspected the complete worktree diff and the new compact/full catalog builders.
- Verified that the generic bridge uses a fixed allow-list and only the fixed
  `/v1/commands/preview` or `/v1/commands/execute` endpoints.
- Verified routine, conditional, and human-only command classification against
  `contracts/v1/commands.json`; unknown commands and incorrect bridges are
  rejected before HTTP forwarding.
- Verified that always-human commands require a browser-issued
  `approvalGrantId`, routine commands reject grants, and the existing server
  remains authoritative for capability, revision, idempotency, warning,
  command decoding, domain, and grant validation.
- Verified exact catalog measurements and legacy-name parity in tests.
- Re-ran `go test ./cmd/baley-mcp ./integration -count=1`,
  `go vet ./cmd/baley-mcp ./integration`, and `git diff --check`; all passed.
- Confirmed the operating worktree retained only its pre-existing untracked
  `debug.log` and that no operating database, service, Codex registration, or
  firewall state was changed.

## Non-blocking residuals

- The compact command catalog returns command names and approval routing on
  demand; command-specific argument decoding remains server-side and detailed
  payload patterns remain in the Baley skill/contracts.
- The full profile intentionally remains large and should only be registered
  for short-lived compatibility or diagnostic sessions.
- Deployment is correctly deferred until a reviewed commit exists because the
  atomic installer rejects an uncommitted dirty worktree.

No commit, merge, push, installation, Task confirmation, or human-only action
was performed by this review.
