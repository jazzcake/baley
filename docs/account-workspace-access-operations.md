# Account and Workspace access operations

Status: Phase 04 operational baseline
Tasks: #135, #134, #133, #132

## Purpose

This runbook enables Baley's local Account, Workspace membership, browser Session,
Agent token, and authenticated human-approval boundary. It does not contain or
generate a default password or token.

## 1. Apply the additive migration

Set `BALEY_DATABASE_URL` for the intended Baley database and run:

```powershell
baley-server migrate up
```

Migration 14 adds the authentication and authorization tables. Applying the
migration does not enable enforcement and does not create credentials.

## 2. Bootstrap the first Owner

`account-bootstrap` opens the same repository used by `serve`, so inject the
stable lease-token secret into the bootstrap process before invoking it. On a
Windows local Pilot where the secret is stored in the current user's environment,
hydrate the process environment explicitly; an already-open terminal does not
automatically inherit later user-environment changes:

```powershell
$env:BALEY_LEASE_TOKEN_SECRET =
  [Environment]::GetEnvironmentVariable("BALEY_LEASE_TOKEN_SECRET", "User")
if ([string]::IsNullOrWhiteSpace($env:BALEY_LEASE_TOKEN_SECRET)) {
  throw "BALEY_LEASE_TOKEN_SECRET is not configured for the current Windows user"
}
```

Use the existing human Actor that should own the Workspace:

```powershell
baley-server account-bootstrap WORKSPACE_ID ACTOR_ID LOGIN_ID "DISPLAY NAME"
```

Enter and confirm the password only at the hidden stdin prompts. Do not put the
password in command arguments, environment variables, shell history, Task Records,
or migration data. Passwords must contain 15–64 Unicode code points.

Before enforced startup, every active Workspace must have at least one active
Account-linked Owner.

## 3. Start the server in enforced mode

Production or Pilot configuration:

```text
BALEY_ENV=production
BALEY_AUTH_MODE=enforced
BALEY_COOKIE_SECURE=true
BALEY_VIEWER_ORIGINS=https://the-exact-viewer-origin.example
BALEY_LEASE_TOKEN_SECRET=<stable external high-entropy secret>
```

Use TLS when `BALEY_COOKIE_SECURE=true`. The server rejects `legacy` authentication
outside development and test environments. On loopback-only development HTTP, set
`BALEY_ENV=development`, choose `BALEY_AUTH_MODE=enforced` explicitly when testing
authentication, and set `BALEY_COOKIE_SECURE=false`.

Every Baley server process sharing a database must receive the same
`BALEY_LEASE_TOKEN_SECRET`. Keep it outside Git, Task Records, logs, and command
arguments. Changing it invalidates active Run lease tokens, so rotate it only when
no Run is active or through a separately planned rotation procedure.

On Windows, setting a user-scoped environment variable updates persistent user
configuration but does not retroactively change the environment of open terminals
or launchers. Hydrate it explicitly as shown in the bootstrap section, or restart
the launcher, before starting each Baley server process.

The login limiter does not trust `X-Forwarded-For` or similar client-supplied
headers. It combines the normalized login ID with the transport peer, so a reverse
proxy's shared loopback address cannot create a global login lockout.

## 4. Build and run the Viewer

Pilot Viewer configuration:

```text
VITE_BALEY_AUTH_MODE=enforced
VITE_BALEY_API_URL=https://the-exact-api-origin.example
```

Viewer builds default to enforced mode when the variable is omitted. Legacy mode
is available only through an explicit `VITE_BALEY_AUTH_MODE=legacy` opt-in for
isolated visual tests that do not exercise Accounts or Workspace membership.

After login, the Viewer lists only Workspaces with an active membership. An Owner
can create a new local Account, attach an existing Account by login ID, change a
Workspace role, transfer ownership, and manage eligible Account credentials.
Account disable and administrator password reset are rejected when the Account has
an active membership in another Workspace; this prevents one Workspace Owner from
changing another Workspace's global Account access.

## 5. Connect an Agent

Install and register the single per-user tokenless Baley MCP Gateway once.
Individual Workspaces do not need separate MCP registrations, gateway tokens,
or Codex threads:

```powershell
.\scripts\install-baley-mcp-windows.ps1
```

The installer builds a versioned executable under `C:\dev-bin\baley\`, runs one
loopback-only Gateway at `127.0.0.1:8090`, and registers Codex with
`http://127.0.0.1:8090/mcp`. It never writes a bearer token or Authorization
header to `config.toml`.

The human logs in, creates or selects a Workspace, and sends its Viewer URL to the
project LLM. The LLM extracts the Workspace UUID and makes its first typed MCP
read. For a new local gateway, Baley returns a short-lived loopback `loginUrl`.
An active Workspace member signs in and explicitly clicks `Connect local
Gateway`. The browser receives a two-minute one-time code, while only the local
gateway that holds the matching pending connection secret can redeem it. Baley
then binds the gateway to that Account. There is no Workspace-specific connection
step, token copy, or Owner-only hand-off. Retrying the same MCP call completes the
connection and stores the device credential in the OS credential manager; the
local file keeps only a key reference. Workspace access is recalculated from the
Account's active membership and role at call time, so newly joined Workspaces do
not require a new thread or schema reload.

The Agent scope is the intersection of the member's Workspace role and the
Agent-safe capability catalog. Owner/Operator receive normal operation scopes;
Viewer/Approver receive read-only scope. Human-only Task confirmation, Gate
passage, and policy changes remain unavailable to it. Raw
tokens never enter chat, `config.toml`, command JSON, Task Records, browser
storage, Git, or logs. Use the loopback Gateway installer and redacted
`baley_mcp_diagnostics` output for Codex access and troubleshooting.

## 6. Confirm a human-only command

1. The Agent fresh-reads the target and prepares an internal typed preview.
2. The Agent presents an outcome-first decision brief in chat. Transport fields such
   as revision, command hash, and capability remain internal unless a mismatch needs
   explanation.
3. The signed-in human opens the Task Inspector (or the equivalent dedicated
   Viewer action), reviews a fresh preview, and explicitly confirms that exact
   command.
4. The browser issues and consumes a short-lived, single-use grant bound to the
   human session, Workspace, command hash, target, snapshot, warnings, and
   revision. The MCP Agent never creates this grant or derives a human Actor.
5. The server rechecks the issuing human's active membership and capability at
   execution time.

No command JSON paste or copied token is needed for ordinary Task confirmation.
Stale revisions, changed hashes or snapshots, revoked roles, cross-Workspace
credentials, expired/reused grants, and Actor mismatches are rejected.

## 7. Recovery and rollback

- Revoke a browser Session with logout; password change/reset or Account disable
  revokes all Sessions for that Account.
- Revoke a compromised Agent token and issue a replacement.
- For local recovery only, stop the server and explicitly select
  `BALEY_ENV=development` with `BALEY_AUTH_MODE=legacy`. Do not use this as a
  production rollback.
- Prefer leaving migrations 14 through 17 in place during rollback. Downgrading
  reintroduces the removed grant schema and can discard current access-control state.
