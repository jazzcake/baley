---
baley_record: 1
record_id: "3dc25795-b345-4031-98d0-21078df97e1a"
task_id: 148
task_key: "oidc-oauth-workspace-mcp"
record_type: independent-agent-review
run_id: "6cbe84ac-b6d0-4c44-8727-d5aa584baf7e"
created_at: "2026-08-28T00:00:00Z"
created_by: "independent-codex-reviewer"
registration_state: pending
supersedes: null
---

# #148 independent OIDC security review

## Verdict

PASS after remediation. No code blocking issue remains.

## Findings and resolution

1. The login page initially selected Google or only the first provider, leaving
   a configured internal provider unavailable when Google was also enabled. It
   now renders every configured provider, with Google first; the Viewer test
   covers the Google plus Keycloak case.
2. Provider configuration accepted an HTTP callback URL. Both issuer and
   redirect endpoint now require HTTPS, a host, and no user info, query, or
   fragment. The OIDC unit test rejects an HTTP callback.
3. The Tailscale Pilot Compose configuration ran in development mode and sent
   non-Secure auth cookies. It now runs as production with only the HTTPS
   Viewer origin and Secure cookies.

## Reviewed boundaries

- External identities are keyed by immutable `(issuer, subject)`; email is
  display metadata, never the Account identity key.
- OIDC uses authorization code + PKCE S256, encrypted verifier storage,
  state/nonce/binding checks, one-time callback consumption, and an HttpOnly
  binding cookie.
- Account link/limited empty-account transfer revokes the retired Account's
  sessions and records audit events. Logout and membership-triggered gateway
  revoke remain in effect.
- OIDC sessions do not add human-only Task confirm, Gate-condition, Gate Task
  pass, or Gate-transition capabilities. #149 remains responsible for removal
  of the local gateway transport token.

## Verification

- `go test ./...` passed.
- `npm test -- --run` passed: 16 files, 87 tests.
- `npm run build` and `docker compose config --quiet` passed.
- The only remaining evidence is a post-deploy Tailscale HTTPS Google
  callback/session/logout smoke test.
