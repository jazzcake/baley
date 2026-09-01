---
baley_record: 1
record_id: "a74aeb95-f851-4c15-8e6d-501e872f5b05"
task_id: 159
task_key: "google-session-longevity"
record_type: independent-agent-review
run_id: "52d0fbc9-eb20-40f7-84b9-0e27d39a7441"
created_at: "2026-09-02T00:20:00+09:00"
created_by: "independent-review-agent"
registration_state: pending
supersedes: null
---

# Google login session longevity independent review

## Final result

- Blocker: 0
- P1: 0
- P2: 0

Approved after review response.

## Findings and disposition

1. The first review found that documentation described all expiry as issue-time policy even though absolute expiry is fixed at issue time while idle expiry slides under the current server policy. The contract now states the exact behavior and absolute cap.
2. The first review requested deterministic lifecycle coverage. Fake-clock tests now cover idle sliding, the absolute cap, exact idle and absolute boundaries, and restart behavior for an existing session.
3. A TLS fake provider now exercises the real OIDC Start/Complete callback path and verifies that it issues a 30-day idle and 90-day absolute Baley session.

## Security boundaries reviewed

- Logout, Account disable, password changes, membership removal, Workspace state, and gateway revocation continue to override the longer session lifetime.
- OIDC and password authentication create the same bounded human session and do not broaden human-only approval capabilities.
- Server restart preserves database-backed sessions; an old short absolute expiry remains fixed and requires one final login.

## Verification reviewed

- non-cached authn, config, HTTP transport, and integration tests: PASS
- final focused authn tests: PASS
- final verdict: approved, no remaining Blocker/P1/P2
