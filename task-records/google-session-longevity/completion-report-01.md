---
baley_record: 1
record_id: "b85c65da-98d0-4c0f-b5e5-06cd03760659"
task_id: 159
task_key: "google-session-longevity"
record_type: completion-report
run_id: "316bb7ac-6901-4456-9315-c5447c788ae9"
created_at: "2026-09-02T11:12:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Google login session longevity completion report

## Outcome

Task #159 is implemented and live-validated. Baley's browser session was the short-lived layer—not Google's OAuth session—and its former hard-coded 30-minute idle and 12-hour absolute limits have been replaced with an explicit, bounded policy.

## Delivered behavior

- Default browser session policy: 30 days idle and 90 days absolute.
- Runtime configuration: `BALEY_SESSION_IDLE_TTL` and `BALEY_SESSION_ABSOLUTE_TTL`, with positive-duration, idle-not-greater-than-absolute, and 365-day maximum-absolute validation.
- Absolute expiry remains fixed at issue time; idle expiry slides under the current runtime policy and never exceeds absolute expiry.
- Password and OIDC callbacks issue the same bounded human session.
- Logout, Account disable, password change, membership removal, Workspace state, and gateway revocation remain stronger immediate invalidation boundaries.
- Existing short-absolute sessions are not silently extended; one fresh login is required after policy deployment.

## Verification

- full Go suite against disposable PostgreSQL migrated through schema 23: PASS
- fake-clock idle renewal, exact idle/absolute boundaries, absolute cap, and restart recovery: PASS
- TLS OIDC discovery/token/signed-ID-token Start/Complete callback path: PASS
- frontend: 16 files / 94 tests PASS
- production frontend build: PASS
- Docker deployment health: ready, schema 23
- public and local OIDC provider endpoint: Google available
- deployed runtime policy: `720h` idle / `2160h` absolute

## Live user validation

After the user performed a fresh Google login, the production database recorded an active Google-backed session issued at 2026-09-02 10:50 KST with approximately 30 days idle lifetime and exactly 90 days absolute lifetime. This closes the prior residual risk from pre-deployment 12-hour sessions.

## Independent review

Approved with no remaining Blocker/P1/P2 after documentation and lifecycle-test findings were resolved.

## Approval boundary

This reports implementation only. Task confirmation remains a separate browser-session human decision.
