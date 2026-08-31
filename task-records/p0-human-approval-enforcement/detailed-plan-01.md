---
baley_record: 1
record_id: "0c93ea77-1e62-45af-9d62-57c879dc8501"
task_id: 158
task_key: "p0-human-approval-enforcement"
record_type: detailed-plan
run_id: "ff8c50df-cbe0-4263-a851-77c482e29b21"
created_at: "2026-08-31T09:00:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# P0 human-only approval enforcement plan

## Security outcome

An Agent bearer credential, MCP request, caller-provided actor field, old
attestation, or completed preview must never be sufficient to exercise a
human-only command. Every such execution consumes a short-lived, single-use
grant issued from an authenticated human browser session after CSRF, active
membership, capability, exact command hash, decision snapshot, and Workspace
revision checks.

## Work sequence

1. Map all human-only commands from the authorization catalog and route every
   execute path through the same planned-command authorization gate. Make Agent
   bearer authentication expose no approval Actor identity.
2. Add an approval-grant persistence model and authenticated human-session API.
   Bind a grant to Workspace, account/actor, command action/entity, canonical
   preview hash, optional decision snapshot, revision, expiry, and single use;
   audit issuance, consumption, rejection, logout/revoke, and expiry.
3. Replace raw MCP/CLI attestation construction with a grant reference. Reject
   legacy body approver IDs, statement hashes, conversation references, and
   timestamps as authority in enforced mode. Keep them only as non-authorizing
   historical audit fields where migration requires it.
4. Remove delegated acceptance auto-confirm. Migrate stored delegated
   assignments to `human_required`; evidence stays evidence and never changes a
   Task to `confirmed` automatically.
5. Add negative integrations for every human-only command using an Agent bearer
   token, forged/stale/replayed/wrong-Workspace grants, logout/revoke/membership
   removal, and body Actor spoofing. Add positive browser-session grant tests.
6. Update the command contract, MCP schemas, operations documentation, and
   diagnostic wording. Build under `C:\\dev-bin\\baley\\`, run focused/full Go
   and frontend suites, deploy, and validate the live approval flow.

## Acceptance boundaries

- Normal Operator work continues tokenlessly through MCP.
- A Task confirm, Gate decision, lane terminal action, Workspace close, or
  policy escalation requires the authenticated human's one-time approval grant.
- No grant, actor ID, or secret is written into Codex config or an environment
  variable. First-device enrollment and ordinary OAuth login remain separate.
