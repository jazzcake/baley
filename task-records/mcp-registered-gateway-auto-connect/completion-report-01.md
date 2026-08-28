---
baley_record: 1
record_id: "acbbc74a-702a-4f70-a705-f4a79d6629f5"
task_id: 142
task_key: "mcp-registered-gateway-auto-connect"
record_type: completion-report
run_id: "0eee5541-2df2-4ee1-808b-6430bf4209c4"
created_at: "2026-08-28T00:00:00Z"
created_by: "codex"
registration_state: pending
supersedes: null
---

# #142 registered MCP gateway auto-connect completion report

## Delivered

- Added durable, Workspace-scoped registrations for one approved local gateway,
  the registered Account member, and its Operator Agent actor. Gateway secrets are
  random, stored server-side only as hashes, and local persistence requires
  encryption.
- A new MCP transport session can renew a Workspace Agent credential with the
  registered gateway credential; the server rechecks the Workspace, active Account
  membership, gateway state, Agent binding, and operator-only scope every time.
- Replacing a gateway, explicit suspected-compromise revoke, Account/member role or
  active-state changes, and Agent membership changes revoke all derived tokens in
  the same transaction. Renewal then requires a new Owner-approved enrollment.
- Added an Owner gateway revoke endpoint and secret-free security audit events.
- Preserved existing human-only Task/Gate approval enforcement. Gateway-issued
  credentials cannot contain human approval or Workspace administration capability.

## Verification

- `go -C server test ./...` passed.
- `npm test -- --run` passed: 16 files, 83 tests.
- `npm run build` passed.
- `docker compose up -d --build api mcp` deployed the change to the local Pilot;
  `/readyz` reported schema version 19 and healthy status.
- A fresh MCP Workspace read succeeded after the gateway restart.
- Independent security review passed after two revoke-path findings were fixed.

## Residual risk

The current HTTP local transport still uses the pre-existing gateway-token protected
loopback layer and encrypted credential store. Replacing that user-visible transport
token with OS-keychain-backed tokenless transport is deliberately deferred to #149;
OIDC login/provider binding is #148. Neither is implemented in this Phase.
