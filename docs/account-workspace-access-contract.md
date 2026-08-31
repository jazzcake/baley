# Account-bound Workspace access contract

Status: Phase 04 implementation baseline
Tasks: #135, #134, #133, #132

## Outcome

Baley identifies a human through a local account and browser session, identifies an
Agent through a Workspace-scoped token, and derives every Actor and authorization
decision from that authenticated principal. A caller-provided Actor ID is audit
metadata at most and never grants authority.

The user-visible membership distinction is Owner versus Participant. Internally,
Participants retain the existing `viewer`, `operator`, and `approver` capability
bundles. Agents can only hold an active `operator` membership.

## Authentication

- Public sign-up, OAuth, MFA, and email recovery are outside this slice.
- A local administrator bootstraps the first Owner without placing a password in
  process arguments, migration data, logs, Events, or Task Records.
- Login IDs are normalized and unique. Passwords are 15–64 Unicode code points.
- Passwords are stored as Argon2id PHC strings with a random salt. The baseline is
  `m=19456`, `t=2`, `p=1`, a 16-byte salt, and a 32-byte output.
- Unknown accounts perform the same bounded Argon2id verification path and return
  the same generic authentication failure.
- Login attempts are bounded before Argon2 execution and rate-limited by digests of
  the normalized login ID and the combined normalized-login/transport-peer tuple.
  A reverse proxy's shared peer address therefore cannot lock every Account at once.
  Forwarded client-IP headers are not trusted. Permanent lockout is not used.
- Browser sessions use at least 256 bits of random entropy. Only token hashes are
  stored. Sessions have idle and absolute expiry and are revoked by logout, password
  change/reset, or Account disable.
- Cookie mutations require the session-bound CSRF value and an exact allowed Origin.
  Production cookies are `__Host-`, `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/`.

## Membership and authorization

- Every Workspace read and write is default-deny after authentication enforcement is
  enabled.
- An active Account and active Workspace membership are both required.
- Workspace roles and capabilities continue to follow
  `contracts/v1/capabilities.json`.
- A Workspace always has at least one active human Owner.
- Last-Owner demotion, removal, and Account disable are rejected under a Workspace
  lock and protected from direct SQL by a database trigger.
- Owner transfer is one atomic operation.
- Cross-Workspace requests are rejected without disclosing protected graph content.
- Background system operations use a reserved internal principal and cannot be
  constructed by external HTTP or MCP requests.

## Agent tokens and human approval

- Agent tokens are Workspace-scoped opaque secrets; only their hashes are stored.
- Agent scopes are a subset of the Operator bundle and can never include approval or
  administration capabilities.
- An Agent bearer never establishes or derives a human approver. Its creator and
  connection approver are credential provenance only.
- A signed-in human uses the Viewer approval surface to preview the exact command and
  issue a five-minute, single-use approval grant. Issuance requires the browser
  session, CSRF token, active Account and Workspace membership, and current command
  capability; Workspace close additionally requires Owner.
- The grant is an opaque UUID reference, not a secret. It is bound to Account, Actor,
  browser session, Workspace, action, entity, Workspace revision, command hash,
  decision snapshot, warning acknowledgement, proceed-reason digest, and expiry.
- Execute locks the Workspace, rechecks every binding and current authority, consumes
  the grant atomically with the command, and records approval and security audit
  evidence. Session or membership revocation invalidates unused grants.
- Enforced HTTP and MCP requests reject legacy body approval fields such as
  `humanApprovalAttestation` and `approvedByActorId`; those fields are historical
  audit storage, never client authority.

## Viewer

- `/login` authenticates without retaining a password in browser state after submit.
- `/workspaces` lists only the current Account's active memberships.
- Workspace routes include the Workspace ID and graph fetches accept it explicitly.
- Switching Workspace aborts old requests, increments a request generation, and
  resets graph, selection, focus, backlog, layout, and viewport state.
- The top bar exposes the Workspace switcher and `Owner` or
  `Participant · <role>`.
- Member administration is a separate audited surface. UI visibility is never the
  authorization boundary.
- Creating a new local Account and attaching an existing Account to another
  Workspace are distinct operations.
- A Workspace Owner cannot disable or reset an Account that has another active
  Workspace membership; Account authority never crosses a tenant boundary.
- The Viewer exposes a human approval panel only to Approver/Owner members. It accepts
  command JSON for a fresh preview and performs issue-and-execute in the same browser
  session; it never displays a plaintext approval secret or asks the user to copy a
  token, header, or environment variable.
- Development traces cover user event, target Workspace, auth state, route, request
  generation, committed graph Workspace, and rendered `data-workspace-id`, without
  credentials or tokens.

## Cutover

Authentication tables are additive and contain no generated default password or
token. `BALEY_AUTH_MODE=legacy` is a temporary local-development compatibility mode
and is rejected outside development and test environments.
`BALEY_AUTH_MODE=enforced` is the Pilot mode and fails startup when an active
Workspace has no active Account-linked Owner. Production Viewer builds default to
enforced mode.

The existing Pilot human Actor is linked only by the explicit bootstrap command.
Legacy approval attestations remain history but cannot authorize new commands in
enforced mode.

## Required negative tests

- unauthenticated and non-member access to every Workspace read/write path
- disabled Account or membership
- Workspace A principal attempting Workspace B access
- forged body Actor and approver IDs
- Agent approval/admin scope escalation
- missing CSRF or invalid Origin on cookie mutations
- login enumeration, oversized input, and rate-limit boundaries
- concurrent last-Owner removal or demotion
- grant revision/hash/snapshot/warning mismatch, expiry, role revocation, and double use
- raw password, session, CSRF, Agent token, and grant absence from DB audit projections
- stale Workspace graph response unable to overwrite the newly selected Workspace
