---
baley_record: 1
record_id: "3f60f35e-7983-4650-9920-af7c74779d77"
task_id: 131
task_key: "gate-public-number-alias"
record_type: independent-agent-review
run_id: "5900819f-864d-44e5-b5f7-cb0b3cd96112"
created_at: "2026-07-30T00:20:00+09:00"
created_by: "codex-independent-review-agent"
registration_state: registered
supersedes: null
---

# Task #131 independent review

## Verdict

PASS. No Blocking, High, Medium, or Low findings remain.

## Findings and response

The first review found one High issue: a new internal Gate ID shaped like
`G#999` could later be shadowed by public Gate number 999. It also found a Low
test gap around concurrent allocation and transaction rollback.

The implementation now reserves canonical `G#[1-9][0-9]*` exclusively for
public Gate references in both application and domain validation. A missing
public reference no longer falls back to an internal ID or alias in the server
or Viewer. Documentation and the Operator skill use the same rule.

The PostgreSQL integration suite now proves that two same-revision Gate creates
serialize to one success and one stale result, that retry receives the next
number, and that a forced insert failure rolls the counter and Workspace
revision back without consuming a number.

## Reviewed boundaries

- deterministic per-Workspace migration backfill and `max(public_id)+1` counter;
- positive, unique, non-reused public IDs and case-insensitive alias uniqueness;
- internal `gateId`, public `G#<n>`, and optional alias resolution;
- Gate create counter CAS and Workspace transaction serialization;
- unchanged Gate approval and capability semantics;
- HTTP, CLI, typed MCP, event evidence, Viewer routes/cards/Inspector, and docs;
- additive Viewer compatibility while an older server omits `publicId`.

## Verification

- `go test ./...`
- `go vet ./...`
- PostgreSQL integration suite with migration 15, concurrent creation, and rollback
- frontend: 14 files, 56 tests
- production TypeScript/Vite build
- Baley Operator skill validator
- `git diff --check`
