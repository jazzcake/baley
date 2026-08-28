---
baley_record: 1
record_id: "d18b433e-6de4-4da5-9ca6-619fdb159db8"
task_id: 148
task_key: "oidc-oauth-workspace-mcp"
record_type: completion-report
run_id: "36d5ecbb-3776-4e88-ac11-c75add0b2c49"
created_at: "2026-08-28T00:00:00Z"
created_by: "codex"
registration_state: pending
supersedes: null
---

# #148 OIDC/OAuth Workspace MCP auto-connect completion report

## Delivered

- Added Google as the default OIDC provider and a configuration-driven generic
  OIDC abstraction for internal Keycloak, Entra ID, Okta, and air-gapped IdPs.
  Google and internal providers can be offered together; an air-gapped
  deployment can configure only its internal TLS provider.
- Baley Account external identities are immutable `(issuer, subject)` pairs.
  Email is not an Account key. An authenticated Account can link multiple
  providers, while the narrow initial-empty-Account migration retires the
  source Account, revokes its sessions, and records an audit event.
- Implemented authorization code with PKCE S256, encrypted one-time state,
  nonce and device/browser binding, callback verification, HttpOnly cookies,
  login/session lifecycle, logout, and audit. Public OIDC callback URLs and
  provider issuers require HTTPS.
- Reused #142's membership-scoped gateway registration and revoke model. OIDC
  only establishes the human session used for connection approval; it does not
  extend an MCP credential beyond the Operator subset or bypass Task confirm,
  Gate condition edits, Gate Task pass, or Gate transition boundaries.
- Hardened the Pilot deployment to production mode, HTTPS-only Viewer origin,
  and Secure authentication/OIDC binding cookies.

## Verification

- `go test ./...` passed.
- `npm test -- --run` passed: 16 files, 87 tests.
- `npm run build` and `docker compose config --quiet` passed.
- Independent security review passed after fixes for multiple configured
  provider selection, HTTPS callback validation, and Secure-cookie deployment.
- Deployed commit `0239562` with `docker compose up -d --build api viewer`.
  `/readyz` reported schema version 20 and the deployed provider endpoint
  reported Google.
- The Workspace owner performed the Tailscale HTTPS smoke test: Google login
  restored the existing Workspace permissions; Logout followed by Google
  login worked again.

## Residual scope

The local MCP HTTP transport still uses the existing gateway token and its
encrypted credential-store key. Removing the user-exposed token and moving the
device secret to the OS Keychain is deliberately deferred to dependent #149.
