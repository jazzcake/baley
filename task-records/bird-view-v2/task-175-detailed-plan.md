---
baley_record: 1
record_id: "17530000-0000-4000-8000-000000000175"
task_id: 175
task_key: "bird-view-v2"
record_type: detailed-plan
created_at: "2026-09-06T07:58:00Z"
created_by: "codex"
registration_state: pending
---

# Task #175 detailed implementation plan

## Boundary diagnosis

A fresh incognito Tailnet context navigated directly to the Bird View URL and was redirected to the login route as expected. Development traces captured the exact schema-27 API session/provider URLs, enforced auth-mode calculation, anonymous React auth state, API provider response, calculated provider list, and rendered login DOM.

The first divergence is the provider boundary: the isolated password-only API truthfully returns an empty OIDC provider list, while the Viewer has no explicitly configured local-review login surface. React preserves the empty list and the DOM consequently renders the no-provider alert. This is a runtime/client review-mode contract gap, not a routing, cookie, database, Tailnet proxy, or stale React-state failure.

## Existing edge grammar

The Workspace graph constructs dependency and Gate edges without an explicit React Flow type, so it uses React Flow's default bezier curve. Bird View currently forces `smoothstep` both when loading graph data and when a human connects a new edge. Task #175 will remove that mismatch and assert the same bezier/default grammar for both paths.

## Work

1. Retain the structured provider-boundary trace and add an explicit, development-review-only local password login calculation controlled by Viewer configuration; keep it disabled by default.
2. Configure only the isolated 8447 Viewer to offer the local review form and verify the existing 8448 password-auth endpoint/account.
3. Make loaded and newly connected Bird View edges use the same default bezier grammar as the Workspace graph, with focused regression assertions.
4. Run focused frontend tests, typecheck, production build, npm audit, and diff checks; backend tests are unnecessary unless backend source changes.
5. Use a fresh Tailnet browser context to prove logged-out redirect, local login, canonical graph, curved loaded/created edges, reload persistence, and trusted double-click focus.
6. Remove temporary browser objects through the durable product command boundary, recheck schema/account/graph counts, register completion evidence, succeed the Run, and report Task #175 implemented without human confirmation.

## Non-goals

No lower Workspace redesign, operating repository/database/service change, firewall mutation, backend auth redesign, OIDC provider invention, destructive integration suite, commit, push, merge, deploy, or human confirmation.
