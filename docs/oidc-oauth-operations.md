# OIDC/OAuth account operations

Baley uses an external identity's immutable `(issuer, subject)` pair as the
login identity. An email claim is requested only as a provider presentation
scope and is never matched to, stored as, or used to link a Baley Account.

## Google (default provider)

Google becomes the default visible provider when all of the following are set
for the API service. Keep client secrets in a mounted secret file, not Compose
text, a Task Record, a terminal history, or a Codex configuration file.

```text
BALEY_GOOGLE_OIDC_CLIENT_ID=<Google OAuth web client ID>
BALEY_GOOGLE_OIDC_CLIENT_SECRET_FILE=/run/secrets/baley_google_oidc_client_secret
BALEY_GOOGLE_OIDC_REDIRECT_URL=https://<baley-api-host>/v1/auth/oidc/google/callback
BALEY_OIDC_STATE_SECRET_FILE=/run/secrets/baley_oidc_state_secret
BALEY_OIDC_POST_LOGIN_URL=https://<configured-viewer-host>/workspaces
```

Register the redirect URL exactly as shown. The API refuses a post-login URL
whose origin is not one of `BALEY_VIEWER_ORIGINS`.

For the local Pilot Compose deployment, create the ignored file
`.tmp/local-pilot/secrets/google_oidc_client_secret` (one secret, newline
allowed) and start the API with:

```text
BALEY_GOOGLE_OIDC_CLIENT_ID=<Google OAuth web client ID>
BALEY_GOOGLE_OIDC_CLIENT_SECRET_FILE=/legacy-secrets/google_oidc_client_secret
BALEY_GOOGLE_OIDC_REDIRECT_URL=https://jazzcake-home.tail87e929.ts.net/api/v1/auth/oidc/google/callback
BALEY_OIDC_POST_LOGIN_URL=https://jazzcake-home.tail87e929.ts.net/workspaces
```

## Internal and air-gapped providers

Add standards-compatible internal providers with `BALEY_OIDC_PROVIDERS`; each
secret must be named by `clientSecretEnv` and sourced using that variable or
its `_FILE` counterpart. Keycloak, Entra ID, and Okta use their issuer URL.
An air-gapped deployment configures only the internal provider entries and no
Google variables.

```json
[
  {
    "id": "internal",
    "label": "Internal SSO",
    "issuer": "https://id.example.internal/realms/baley",
    "clientId": "baley-web",
    "clientSecretEnv": "BALEY_OIDC_INTERNAL_CLIENT_SECRET",
    "redirectURL": "https://baley-api.example.internal/v1/auth/oidc/internal/callback",
    "scopes": ["openid", "profile", "email"]
  }
]
```

## Security and operations

- Authorization code with PKCE S256, state, nonce, and an HttpOnly browser
  device-binding cookie is required. The server stores only hashes plus an
  encrypted short-lived verifier.
- The callback validates discovery metadata, token signature, issuer, client
  audience, nonce, one-time state, expiry, and device binding before issuing
  the ordinary bounded Baley session.
- Use the Account menu's **Google account link** or internal-provider link
  action while already authenticated to attach another `(issuer, subject)` to
  that same Account. Email cannot authorize linking.
- If an initial OIDC sign-in created an empty Account, linking that identity
  from an established Account safely retires the empty Account and preserves
  the established Account's memberships and roles. The transfer is permitted
  only when the source has no password, no active membership, and no other
  external identity; it revokes the source sessions and leaves an audit event.
- When moving from a local-password Account, sign in with the OIDC identity,
  then use **기존 Baley Account 권한 이전** from the Account menu. The legacy
  password is used only to prove ownership for that one migration; after it
  succeeds, the browser reauthenticates with the OIDC provider and the normal
  `(issuer, subject)` linking rules perform the transfer. The public login
  screen remains OIDC-only.
- Logout revokes the active Baley session. Account disable, membership changes,
  and local-gateway revocation retain the #142 gateway/session invalidation
  behaviour. OIDC does not issue an Agent credential with human approval
  capabilities; Task confirmation, Gate changes, Gate Task pass, and Gate pass
  remain human-only.
- Audits record provider and a non-reversible external-identity digest, never
  client secrets, PKCE verifiers, access tokens, ID tokens, or email claims.

For a rollback, remove a provider's configuration and restart the API. Existing
local-password sessions remain available; existing external identity records
are retained as audit/account-link history and cannot be used while the
provider is absent.
