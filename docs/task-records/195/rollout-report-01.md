---
baley_record: 1
record_id: "d3101cbc-c80d-48fc-b684-c530f057de18"
task_id: 195
record_type: completion-report
run_id: "e1d93019-dba9-4433-a863-b93177744b3b"
created_at: "2026-09-18T14:51:49+09:00"
created_by: "codex"
supersedes: "0c6e0797-8adf-49ea-8c34-6c6ac3188e49"
status: completed
---

# Task #195 rollout report

## Outcome

The compound conversational Task confirmation fix is integrated, pushed, and
deployed. Baley API now serves commit
`2d5e3cbad0b48ed0e7ea5e3bae02a45cee424522` as version `task-195` on schema
29. The personal Baley Codex plugin was rebuilt and installed as
`0.1.0+codex.20260918054702`, including the compound-decision grammar and the
target-generated decision UUID contract.

## Root cause

The original `decision_evidence_mismatch` did not come from a failed external
Codex thread, turn, or message binding. The server did not implement such a
provider-ID comparison. The statement validator rejected the request first:

- `all_awaiting_confirmation` rejected statements containing numbered Tasks;
- `task` scope accepted only a closed single-target sentence;
- `decisionId` required a UUID, so a `msg_...` identifier was invalid even
  though UUID-shaped thread or turn identifiers passed the format check.

The fix parses a conservative comma-delimited compound statement, assigns every
listed Task to an explicit action, and accepts `task.confirm` only for Tasks in
the confirmation group. It also returns a safe field-level mismatch reason.

## Integration and verification

- Rebased the 14-commit Task #186-#195 line onto the latest Baley/dim0 operating
  branch with no conflicts and preserved the unrelated untracked `debug.log`.
- Focused compound-decision tests passed.
- Full Go suite passed with only the two tests that intentionally push remote
  `main` excluded; `go vet ./...` passed.
- Pushed the exact deployment SHA atomically to `main`,
  `jazzcake/mcp-login-membership-auth`, and
  `jazzcake/task-195-compound-decision`.
- API image `sha256:f2def2040b366898fdd5265c99bb44ac97804916013d5d313c13433b387cef61`
  carries the exact revision and schema-29 labels.
- Local API, Viewer `/api/versionz` proxy, and Tailnet `/api/versionz` all
  returned version `task-195`, the exact commit, and schema 29. The API
  container is healthy; ports 8080, 5174, and 8090 remain loopback-bound.
- Live MCP reads of DayTripper Tasks #61 and #62 succeeded after deployment;
  both remained eligible and `implemented` before any human decision.

## Deployment recovery note

The first API recreation inherited the obsolete process value
`BALEY_GOOGLE_OIDC_CLIENT_SECRET_FILE=/legacy-secrets/google_oidc_client_secret`
and failed health checks. The standard secret was still present and correctly
mounted at `/run/secrets/baley_google_oidc_client_secret`. The container was
immediately recreated with that canonical in-container path and returned
healthy. Goose reported schema 29 with no migration to run; no database schema
or Task status was changed by this recovery.

## Human boundary

No Task was auto-confirmed merely to prove deployment. An attempted positive
DayTripper #61 canary was stopped by the agent safety boundary before Baley
execution because the current user instruction was not itself a direct #61
confirmation. #61 remains `implemented`. A future explicit compound decision
can now be handled by the deployed parser without another server or plugin
change.
