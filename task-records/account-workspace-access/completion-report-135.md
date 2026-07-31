---
baley_record: 1
record_id: "4b4797a6-764d-49ae-9390-20035e3483c7"
task_id: 135
task_key: "local-account-session-authentication"
record_type: completion-report
run_id: "da101271-e685-4ee3-ae1f-e203f57ba3c8"
created_at: "2026-07-28T01:42:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Task #135 completion report

Implemented local login IDs and Argon2id credentials, hidden-stdin Owner bootstrap,
bounded verification and rate limiting, hash-only browser Sessions, CSRF and Origin
checks, idle/absolute expiry, logout, password change/reset, Account disable, and
Session revocation. Production and staging reject legacy authentication and
insecure cookies.

Validation: full Go tests, real PostgreSQL migration/integration, `go vet`,
frontend 49 tests, production build, formatting, secret scan, and diff check PASS.
Independent security re-review: PASS.

Deployment was intentionally not performed. Migration 14 and the first real Owner
bootstrap require an operator-selected Account ID/password and cutover window.
