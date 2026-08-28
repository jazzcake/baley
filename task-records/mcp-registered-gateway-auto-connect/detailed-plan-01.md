---
baley_record: 1
record_id: "f2d96873-64b3-4c97-8c05-a88b1e42e884"
task_id: 142
task_key: "mcp-registered-gateway-auto-connect"
record_type: detailed-plan
run_id: "a6684dbf-fbcd-4375-b952-dde933e88e72"
created_at: "2026-08-28T00:00:00Z"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Registered Workspace-user MCP auto-connect plan

## Objective

Allow a previously registered, local-only MCP gateway to resume a Workspace-scoped
Operator connection in a new MCP transport session without another Owner approval.
The server remains the source of truth for the registered gateway, user membership,
Workspace scope, and credential validity.

## Design and safety invariants

- Persist gateway registration as a Workspace, Account, and gateway-identity binding;
  never infer registration from a transport bearer or an arbitrary Agent actor ID.
- Bind each issued Agent credential to the registration generation. Each request must
  verify the active Account membership, gateway status, Workspace scope, and current
  generation before use.
- On membership removal or role revocation, local gateway replacement, explicit
  revocation, or suspected compromise, invalidate all bound Agent credentials in the
  same transaction and write a security/audit event.
- Keep the existing manual connection request as the enrollment/recovery fallback.
  A registration can be created only after that trusted approval path succeeds.
- Preserve the existing Agent operator-only scope. No connection shortcut may add
  Task confirmation, Gate condition, Gate pass, or other human-only authority.

## Implementation steps

1. Trace the current connection-request, Agent-token issuance, membership mutation,
   and MCP credential-store paths; identify the first authorization boundary that
   currently loses gateway identity across a transport restart.
2. Add durable registered-gateway and credential-generation persistence plus atomic
   revocation helpers. Extend connection consume to enroll the approved local gateway.
3. Make the MCP client present a stable locally generated gateway identity when it
   requests/resumes a Workspace connection, and persist only the resulting
   Workspace-scoped Agent credential.
4. Add server-side automatic issuance for an active registered binding and explicit
   invalidation on membership and gateway lifecycle changes. Keep audit data secret-free.
5. Cover new-session recovery, unregistered gateway denial, Workspace isolation,
   membership removal, gateway replacement/revocation, and human-only authority
   regression with unit and PostgreSQL integration tests.
6. Run Go, frontend, and repository verification. Record independent review and
   completion evidence before reporting #142 implemented.

## Phase 07 planning boundary

#148 and #149 will not be implemented until #142 is complete and G#6 passes. Their
future detailed plans will build on this durable gateway registration rather than
reuse a transport token as identity.
