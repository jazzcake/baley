---
baley_record: 1
record_id: "c32f68c5-0b9f-4188-8dd4-c380f6b931cd"
task_id: 163
task_key: "mcp-loopback-only-security"
record_type: review-response
run_id: "fdaae324-3af4-4889-a837-5fd50d993093"
created_at: "2026-09-02T17:07:01+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# #163 review response

## Outcome

All independent-review findings were resolved and the independent re-review
passed without a remaining blocker.

## Changes

- Standardized logout, link Begin, and callback redeem on a
  request-before-session database lock order.
- Added fail-closed session-expiry and Actor/session mismatch tests.
- Added membership-removal-before-redeem coverage.
- Added repeated real concurrent logout/redeem coverage, including immediate
  invalidation of a token when redeem wins the race.
- Added migration 25 up/down coverage for safe preservation and unsafe legacy
  link deletion.

## Verification

- Focused PostgreSQL security tests passed.
- The complete PostgreSQL integration package passed.
- The concurrency scenario passed ten repeated test executions.
- Full Go tests and `go vet` passed.
- Frontend 107 tests and the production build passed.
- PowerShell AST validation and `git diff --check` passed.
- Independent re-review returned PASS.

