---
baley_record: 1
record_id: "6edc21f6-03cb-4d7d-9d95-5e67212b6b4d"
task_id: 195
record_type: completion-report
run_id: "19a137cc-3ca5-4f15-bb8b-816aabe8d540"
created_at: "2026-09-18T17:20:00+09:00"
created_by: "codex"
supersedes: "1f6c9a42-7d35-4b80-a164-2e8f5c3d9071"
status: implemented
---

# Task #195 natural-language Task decision boundary

Task #195 now keeps natural-language interpretation in the Agent and typed
command enforcement in Baley. The server no longer reparses the user's
verbatim statement with a closed comma, particle, ordering, or Task-number
grammar. The Agent declares the explicit human decision as `scope`, `action`,
and `taskId`; Baley continues to bind that declaration to the linked human
capability, exact command target, current Workspace revision, canonical command
hash, per-target UUID decision ID, idempotency key, and immutable statement
hash.

This fixes the product-boundary defect exposed by the sentence
`이제 #61, #62 삭제해줘.`. The prior implementation treated the audit statement
as a second command language and returned `decision_evidence_mismatch
(statement_scope)` unless the human followed a narrow comma-delimited grammar.
That contradicted the existing contract statement that the Agent declares the
typed meaning and the server does not authenticate external conversation
semantics.

Verification completed:

- focused conversational decision tests: PASS
- `go test ./... -skip '^TestCommandRemoteGitVerifier' -count=1`: PASS
- `go vet ./...`: PASS
- JSON contract parsing: PASS
- implementation commit `70ee961b36338632d1014be5e5fdcea7eae6e43f`
  atomically pushed to `main`, the operating branch, and the Task branch
- API deployed as `task-195-natural-decisions`, schema 30, commit `70ee961`;
  local API, Viewer proxy, and Tailnet version/readiness probes agree
- personal plugin reinstalled as `0.1.0+codex.20260918081712`; the installed
  Skill contains the natural-language decision contract
- no MCP wire or executable change was required; the existing gateway executed
  both conversational discard commands against the updated API

Live DayTripper proof used the exact statement `이제 #61, #62 삭제해줘.`:

- Task #61: `implemented -> discarded`, command
  `9aeb2f9c-b898-44db-940b-e8559deb8fa2`, events
  `6a272181-1dad-493f-9296-432702d3961c` and
  `260f1920-1cbc-4f57-b753-ac3f58d65a05`
- Task #62: `implemented -> discarded`, command
  `623ba2ef-b7cf-44b6-bd47-f65014534c63`, events
  `5892af2a-92ac-476e-9fff-a7c074f08c6a` and
  `fb8084b8-f1a6-4ea6-97c1-73d67e978346`
- final DayTripper Workspace revision: 1036

Both commands used `linked_account_conversation`, unique target-bound decision
UUIDs, fresh previews, and no Viewer approval grant. The unrelated untracked
`debug.log` remained untouched.
