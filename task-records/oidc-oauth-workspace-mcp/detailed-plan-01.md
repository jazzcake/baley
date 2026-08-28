---
baley_record: 1
record_id: "8bd940d5-0f38-4db0-bb86-f752e6f21811"
task_id: 148
task_key: "oidc-oauth-workspace-mcp"
record_type: detailed-plan
run_id: "9babdc8f-dccd-43b8-8578-325ce51c5995"
created_at: "2026-08-28T00:00:00Z"
created_by: "codex"
registration_state: pending
supersedes: null
---

# OIDC/OAuth Workspace MCP auto-connect plan

## Boundary

This is planning only. Implementation is blocked until #142 is implemented,
confirmed by a human, and G#6 passes.

## Architecture

- Represent a Baley Account's external identities with the immutable pair
  `(issuer, subject)`, never email. Store normalized issuer metadata separately
  from displayed email/name claims.
- Add a provider abstraction with Google as the default hosted provider and a
  generic OIDC discovery/JWKS implementation for Keycloak, Entra ID, Okta, and
  air-gapped internal IdPs. Google must be optional in an air-gapped deployment.
- Use authorization-code flow with PKCE, exact redirect validation, state/nonce
  binding, callback expiry and one-time consumption. Bind local gateway device
  enrollment to the authenticated Account and reuse #142's Workspace/gateway
  registration rather than issuing a browser credential to MCP.
- Account linking requires an authenticated existing Baley Account and recent
  reauthentication from both sides; matching email alone never links identities.
- Logout, IdP/session revocation, membership change, gateway revoke, and
  compromised-device recovery all revoke the relevant derived MCP credentials and
  append audit events without tokens or identity assertions.
- OAuth automates only connection establishment. Agent credentials retain the
  operator-only capability subset, so Task confirm and all Gate human-only actions
  remain attestation-gated.

## Delivery slices

1. Data migrations and provider configuration validation, including offline-only
   internal-provider startup mode.
2. OAuth authorization/callback/token validation and `(issuer, subject)` Account
   identity binding/linking APIs with PKCE and audit tests.
3. Local gateway browser handoff that turns a successful login into a #142 gateway
   registration and short-lived Workspace Agent credential; support logout/revoke.
4. Viewer login/linking/recovery UX, development instrumentation of event, account,
   gateway, request, and rendered state (never credentials), plus integration and
   provider contract tests.

