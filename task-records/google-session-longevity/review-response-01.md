---
baley_record: 1
record_id: "812c82be-3bcc-4abe-8374-86faa372b07a"
task_id: 159
task_key: "google-session-longevity"
record_type: review-response
run_id: "d4448ad1-7ebc-4c8b-8add-617b5ebbabaa"
created_at: "2026-09-02T00:20:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Google login session longevity review response

## Response

All documentation and lifecycle-test findings from the initial independent review were accepted and resolved.

## Changes made

- Corrected the session contract to distinguish fixed absolute expiry from current-policy sliding idle expiry.
- Added fake-clock coverage for renewal, caps, exact expiry boundaries, and restart behavior.
- Added a real OIDC Start/Complete callback-path test with TLS discovery, token exchange, signed ID token verification, and long-lived session assertions.

## Re-verification

- `go test -count=1 ./internal/authn`: PASS
- independent re-review: approved, Blocker/P1/P2 all zero
