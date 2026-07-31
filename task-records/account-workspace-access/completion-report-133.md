---
baley_record: 1
record_id: "5642fc99-9634-45b5-9aeb-31ca532b76ff"
task_id: 133
task_key: "authenticated-approval-agent-token"
record_type: completion-report
run_id: "52b5d0fb-84e3-442e-a33d-4415c2610947"
created_at: "2026-07-28T01:42:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Task #133 completion report

Implemented Workspace-scoped Agent tokens, restricted scopes, MCP bearer transport,
authenticated human approval grants, fresh preview binding, atomic one-use
consumption, revocation, expiry, exact warning/proceed-reason repetition, Actor
provenance, and credential-bound idempotent retry.

The end-to-end PostgreSQL test covers browser Session grant issue through Agent
bearer execute. The Viewer presents a complete one-time MCP execute input to an
Owner or Approver without persisting its token.

All Go, PostgreSQL, frontend, build, format, secret, and diff validations PASS.
Independent security re-review: PASS.
