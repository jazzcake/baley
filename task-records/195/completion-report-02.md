---
baley_record: 1
record_id: "c9d3b74e-aea7-4293-83f2-703d2d0b83cb"
task_id: 195
record_type: completion-report
run_id: "d0c574db-8f6f-47ed-b830-25f88abae3f2"
created_at: "2026-09-18T14:57:25+09:00"
created_by: "codex"
supersedes: "d3101cbc-c80d-48fc-b684-c530f057de18"
status: immutable_correction
---

# Task #195 verified rollout evidence

This immutable correction places the final Task #195 completion Record under
the repository-configured `task-records` root. It supersedes the rollout Record
whose content was correct but whose `docs/task-records` location could not be
verified by Baley's remote Git provider.

The original failure was a statement/scope grammar rejection, not an external
Codex conversation-ID binding failure. The deployed fix conservatively assigns
every comma-delimited numbered Task to an explicit confirm or discard action,
uses target-generated UUIDs for `decisionId`, treats `conversationRef` as opaque
audit provenance, and reports a safe field-level mismatch reason.

Commit `2d5e3cbad0b48ed0e7ea5e3bae02a45cee424522` is deployed as Baley API
version `task-195` on schema 29. Local API, Viewer proxy, and Tailnet version and
readiness checks passed; the container is healthy and ports 8080, 5174, and
8090 remain loopback-bound. The full Go suite passed with only two tests that
intentionally push remote `main` excluded, and `go vet ./...` passed.

The personal Baley plugin was rebuilt and installed as
`0.1.0+codex.20260918054702`. Its installed `baley-manage-work` Skill contains
the compound-decision grammar and target-generated decision UUID guidance, so
new DayTripper Codex sessions can use the corrected flow without a DayTripper
repository change.

The first API recreation inherited an obsolete OIDC secret-file path and failed
health checks. The canonical mounted path was restored immediately; Goose
reported schema 29 with no migration to run, and the replacement API returned
healthy. No Task was auto-confirmed as a canary. DayTripper #61 remains
`implemented` and eligible for a future explicit human confirmation.
