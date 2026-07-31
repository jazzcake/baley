---
baley_record: 1
record_id: "ea9bfee4-35e3-4298-8cdb-7e16356bcea1"
task_id: 135
task_key: "account-workspace-access"
record_type: detailed-plan
run_id: "fe305f4b-4c31-40a2-b137-89471dd613a5"
created_at: "2026-07-28T00:16:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Account-bound Workspace access detailed plan

This plan implements Tasks #135, #134, #133, and #132 as one reviewed vertical
slice. The four Tasks are parallel acceptance outcomes after #130 and all converge
on #123; their code is implemented in dependency order without inserting human
confirmation pauses between them.

## 1. Contract and migration

1. Add the normative contract in `docs/account-workspace-access-contract.md`.
2. Add migration 00014 for Accounts, credentials, sessions, login throttles,
   Workspace memberships, Agent tokens, approval grants, and security audit events.
3. Preserve existing Actor, command, Event, and attestation history.
4. Add direct-SQL constraints/triggers for Actor kind, Agent role, append-only
   security audit, and last active human Owner.
5. Add an explicit local bootstrap command; never create a default password/token in
   migration or seed data.

## 2. Task #135 — local Account and Session authentication

1. Implement bounded Argon2id PHC encoding, parsing, verification, and rehash checks.
2. Implement Account lookup, generic failed authentication, bounded DB-backed rate
   limiting, and login success/failure audit.
3. Implement opaque session and CSRF values with hash-only persistence, idle and
   absolute expiration, rotation, logout, password change, and revocation.
4. Add `/v1/auth/login`, `/v1/auth/logout`, `/v1/me`, and `/v1/me/password`.
5. Add strict cookie, Origin, CSRF, CORS credential, and no-store response behavior.
6. Implement transactional first-Owner bootstrap from protected stdin.

## 3. Task #134 — membership and authorization

1. Persist active Workspace memberships with viewer/operator/approver/owner roles.
2. Link the existing `authz` catalog and policy to authenticated HTTP/MCP principals.
3. Default-deny every Workspace read and write in enforced mode.
4. Derive initiated/executed Actor provenance from the principal; reject mismatches.
5. Add member list/add/change/deactivate/reactivate and atomic Owner transfer APIs.
6. Recheck membership and dynamic command capability inside the locked command path.
7. Preserve an internal system-principal path for the Run expiry sweep.

## 4. Task #133 — Agent token and authenticated approval

1. Issue, list, revoke, and rotate Workspace-scoped Agent tokens; return raw material
   only once and persist only hashes.
2. Add Bearer authentication to HTTP and `BALEY_AGENT_TOKEN` support to MCP.
3. Issue approval grants only from authenticated human sessions after a server-side
   fresh preview and capability check.
4. Bind grants to the full decision and warning surface and consume them atomically
   with command execution.
5. Keep idempotent replay safe while rejecting grant reuse by a different command.
6. Replace new audit-only approval execution with
   `authenticated_approval_grant`.

## 5. Task #132 — Workspace and member UI

1. Add typed auth/workspace API clients with `credentials: include`.
2. Add auth bootstrap, login, Workspace chooser, Workspace-scoped routes, and account
   controls.
3. Make graph fetch accept a Workspace ID; remove the environment ID as the normal
   selection authority.
4. Key the Viewer by Workspace and abort/generation-guard polling and layout state.
5. Add the top-bar Workspace/role controls and Owner-only member administration.
6. Add development-only structured traces at the event/state/request/DOM boundaries.

## 6. Verification

- focused domain/auth/session/membership/grant tests
- PostgreSQL migration, bootstrap, last-Owner race, token/grant race integration
- complete HTTP route and capability matrix including negative paths
- MCP Bearer and approval-grant end-to-end
- React auth, routing, switch race, role UI, session expiry, and member conflict tests
- full `go test ./...`, `go vet ./...`, frontend tests, production build, and
  `git diff --check`
- independent backend, security, and Viewer review; all blocking findings resolved

## 7. Completion and authority

Each Task receives its own implementation assessment and completion report reference.
All four Tasks remain `implemented` until one grouped human confirmation. #123 and
#124 are not started by this implementation session. The Pilot entry Gate remains a
separate human-only decision.
