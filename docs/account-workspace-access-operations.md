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

Register the Baley MCP loader once. Individual Workspaces do not need separate MCP
registrations, environment files, or Codex threads:

```toml
[mcp_servers.baley]
command = "powershell.exe"
args = ["-NoProfile", "-ExecutionPolicy", "Bypass", "-File", 'D:\Project_AI\baley\scripts\run-baley-mcp.ps1']
```

The human logs in, creates or selects a Workspace, and sends its Viewer URL to the
project LLM. The LLM extracts the Workspace UUID and makes its first typed MCP
read. For an unknown Workspace, Baley returns a short-lived approval URL. The
logged-in Workspace Owner opens it and approves one Operator connection. Retrying
the same MCP call completes the connection and stores the Workspace-scoped
credential in the OS credential manager; the local file keeps only a key
reference. The store is read at call time, so new Workspace connections do not
require a new thread or schema reload.

The granted identity has only the Operator capability catalog. Human-only Task
confirmation, Gate passage, and policy approval remain unavailable to it. Raw
tokens never enter chat, `config.toml`, command JSON, Task Records, browser
storage, Git, or logs. `scripts/prepare-local-pilot-agent.ps1` remains only as a
manual recovery/rotation tool; it is not the normal onboarding path.

## 6. Approve a human-only Agent command

1. The Agent fresh-reads the target and prepares an internal typed preview.
2. The Agent presents an outcome-first decision brief in chat. Transport fields such
   as revision, command hash, and capability remain internal unless a mismatch needs
   explanation.
3. The human approves or rejects that specific outcome in the same conversation.
4. On approval, the Agent immediately creates a command-specific chat attestation,
   repeats exact warning acknowledgements when needed, and executes the MCP command.
5. The server derives the approving Actor from the human who connected the current
   Agent credential and rechecks that person's active Workspace role and capability.

No Viewer panel, command JSON paste, grant issuance, token copy, or second approval
channel exists. Stale revisions, changed hashes or snapshots, revoked roles,
cross-Workspace credentials, and approval-Actor mismatches are rejected.

## 7. Recovery and rollback

- Revoke a browser Session with logout; password change/reset or Account disable
  revokes all Sessions for that Account.
- Revoke a compromised Agent token and issue a replacement.
- For local recovery only, stop the server and explicitly select
  `BALEY_ENV=development` with `BALEY_AUTH_MODE=legacy`. Do not use this as a
  production rollback.
- Prefer leaving migrations 14 through 17 in place during rollback. Downgrading
  reintroduces the removed grant schema and can discard current access-control state.
