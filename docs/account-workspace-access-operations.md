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

Production Viewer builds default to enforced mode when the variable is omitted.
Development builds retain legacy mode only for local compatibility.

After login, the Viewer lists only Workspaces with an active membership. An Owner
can create a new local Account, attach an existing Account by login ID, change a
Workspace role, transfer ownership, and manage eligible Account credentials.
Account disable and administrator password reset are rejected when the Account has
an active membership in another Workspace; this prevents one Workspace Owner from
changing another Workspace's global Account access.

## 5. Connect an Agent

An Owner issues a Workspace-scoped Agent token. Configure it only in the Agent/MCP
process:

```text
BALEY_AGENT_TOKEN=<opaque token>
```

For a local Codex stdio registration, store `BALEY_SERVER_URL` and
`BALEY_AGENT_TOKEN` as Windows User environment variables and whitelist their
names rather than copying their values into `config.toml`:

```toml
[mcp_servers.baley]
command = "go"
args = ["-C", 'D:\Project_AI\baley\server', "run", "./cmd/baley-mcp"]
env_vars = ["BALEY_SERVER_URL", "BALEY_AGENT_TOKEN"]
```

After issuing or rotating a token, completely exit and relaunch the Codex host so
it receives the current User environment, then open a new thread. Restarting only
the MCP child or opening a thread under an already-running host can preserve a
stale environment. If the client cached an older tool schema, the new thread also
loads the current `approvalGrantToken` execute field. Never put the Agent token in
the static `env` table, command arguments, command JSON, Task Records, Git, browser
storage, or logs.

Rotation is issue-new, update the Agent process, verify it, then revoke-old.

## 6. Approve a human-only Agent command

1. The Agent prepares the typed command without an approval token.
2. The authenticated human opens the Viewer approval panel and pastes the command.
3. The server recomputes a fresh preview and shows the exact target, revision,
   capability, command hash, projected diff, warnings, and decision snapshot.
4. The human acknowledges every warning and provides a proceed reason when needed.
5. The Viewer issues a short-lived, one-use grant and displays a complete typed MCP
   execute input once. This input contains `approvalGrantToken` plus the exact
   `acknowledgedWarningCodes` and `proceedReason` bound to the grant.
6. Copy that complete input into the matching MCP execute call. Do not omit or
   change the warning acknowledgement fields: a common `dangling_path` grant will
   fail when execute does not repeat the same codes and proceed reason.

The grant is valid only for the exact previewed outcome. Stale revisions, changed
hashes or snapshots, expired grants, revoked roles, cross-Workspace use, and reuse
are rejected.

## 7. Recovery and rollback

- Revoke a browser Session with logout; password change/reset or Account disable
  revokes all Sessions for that Account.
- Revoke a compromised Agent token and issue a replacement.
- Revoke an unused approval grant from the Viewer.
- For local recovery only, stop the server and explicitly select
  `BALEY_ENV=development` with `BALEY_AUTH_MODE=legacy`. Do not use this as a
  production rollback.
- Prefer leaving migration 14 in place during rollback. Its tables are additive,
  and downgrading after credential data exists discards the new access-control
  state.
