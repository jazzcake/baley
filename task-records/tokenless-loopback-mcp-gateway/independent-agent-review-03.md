---
baley_record: 1
record_id: "7d3fabb3-3021-46bd-9b55-b1afde9dbbe2"
task_id: 157
task_key: "tokenless-loopback-mcp-gateway"
record_type: independent-agent-review
run_id: "4f39de1e-0f4e-4249-b4e0-9d952dda00d3"
created_at: "2026-08-30T17:35:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: "bb1a0e75-0585-4a7c-b208-5074c4900be3"
---

# Independent review: legacy approval authority in enforced mode

## Verdict

Blocking finding: human-only authority is still exercisable through the legacy
connected-human attribution path when the caller holds an Agent bearer token.

## Evidence

1. `authn.Service.AuthenticateBearer` sets an Agent principal's
   `ApprovalActorID` to the token's historical `created_by_actor_id`.
2. `application.Service.Execute` overwrites a supplied attestation's approver
   with that stored value. Its subsequent check requires only that the value
   names a known human Actor; it does not require a current authenticated human
   session or a fresh human-bound approval assertion.
3. The HTTP command route authorizes only Workspace tenancy/read access before
   application execution. It does not apply the `authz.AuthorizePlannedCommand`
   human-session constraint on the executed command.
4. The existing integration test explicitly treats an HTTP request carrying an
   Agent bearer token and a computed attestation as a successful
   `task.confirm`; the test passed unchanged. The same preview endpoint returns
   the command and decision hashes required to construct an attestation.

## Impact

An actor with a valid MCP/Agent credential can construct an attestation for the
Account that originally created the token and invoke human-only operations such
as Task confirmation and active Gate pass without that human's current-session
approval. This remains reachable with `BALEY_AUTH_MODE=enforced`; it is not
limited to the explicit development `legacy` runtime mode.

## Required remediation

Remove `ApprovalActorID` derivation from bearer authentication and reject any
human-approval attestation unless it is bound to a currently authenticated
human session (or an equivalent server-verifiable, short-lived approval grant).
The command route must apply the planned-command authorization decision at
execute time. Update the positive legacy test to a rejection regression and add
negative coverage for Task confirm, active Gate mutation, Gate pass, discard,
lane close, and Workspace close through an Agent bearer token.
