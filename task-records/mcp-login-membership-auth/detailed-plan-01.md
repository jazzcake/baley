---
baley_record: 1
record_id: "266d9159-c939-4f4b-9ca3-962fe41a29e7"
task_id: 162
task_key: "mcp-login-membership-auth"
record_type: detailed-plan
run_id: "be407741-b3ed-4ec1-9191-d45c94717876"
created_at: "2026-09-02T12:15:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# #162 MCP login and membership authorization plan

## Easy explanation

MCP device onboarding will be described and implemented as a login-bound link,
not as a separate Workspace approval. Once the browser Account is signed in,
the server derives access from that Account's active membership and role.

## Why it is needed

The current page already links automatically after authentication, but runtime
types, URLs, database states, diagnostics, Skills, and operations documents
still use `approve`, `approvalUrl`, `approved`, and `/mcp-connect`. That legacy
vocabulary implies an independent authorization decision that no longer exists.

## What changes when complete

1. Rename the first-device hand-off to MCP login linking throughout the local
   credential store, MCP errors, HTTP routes, Viewer component, API client, and
   database projection.
2. Replace approve/reject persistence with a single authenticated link action;
   expired or abandoned requests disappear without a rejection decision.
3. Derive issued Agent scopes from the signed-in member's role per Workspace.
   Owner/Operator receive ordinary Operator scopes, while Viewer/Approver are
   restricted to read-only Agent scopes; human-only capabilities are never
   copied into Agent credentials.
4. Preserve device proof, OS Keychain storage, loopback-only transport, and
   immediate invalidation on logout, membership change, archive, replacement,
   suspected compromise, or explicit revoke.
5. Migrate schema and credential-store metadata without persisting bearer
   tokens or requiring an environment token.
6. Update canonical Skills and all current product/operations documentation so
   MCP login linking is not called approval. Historical human Task/Gate approval
   documentation remains because it describes a different enforced boundary.

## Scope and exclusions

- Included: Go server and MCP client, PostgreSQL migration, React Viewer/API,
  focused and full tests, plugin validation/install, deployment, and live smoke.
- Excluded: weakening or deleting Task/Gate/Lane/Workspace human-only approval
  grants, allowing an Agent token to inherit human approval capabilities, and
  exposing the loopback Gateway outside the local machine.

## Verification

- Regression tests for first-device login link, all role mappings, membership
  removal, logout, archive, gateway replacement/revoke, stale and expired links,
  credential-store migration, and concurrent resumes.
- Frontend component/API tests, TypeScript typecheck, production build, all Go
  tests including PostgreSQL integration, migration 1 through the new schema,
  independent review, Docker deployment health, OIDC provider check, and a
  deployed login-link asset/API smoke.
- Targeted repository scan must find no MCP-specific approval terminology or
  route. Any remaining approval occurrence must belong to the explicit
  human-only Task/Gate/Lane/Workspace security model.
