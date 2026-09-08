---
baley_record: 1
record_id: "3a5a1101-b084-4915-b546-e88fc59a52c1"
task_id: 181
task_key: "mcp-tool-catalog-rework"
record_type: detailed-plan
run_id: "40c51917-c73c-4cea-ac1d-ab4c176badea"
created_at: "2026-09-07T07:07:42Z"
created_by: "codex"
registration_state: pending
supersedes: null
---

# #181 detailed plan: typed Task rework tools

## Objective

Restore exact typed `baley_task_rework_preview` and
`baley_task_rework_execute` tools on both MCP profiles without changing the
existing `task.rework` domain command. Keep the compact endpoint at no more
than 15 tools by moving `baley_workspace_get` to the full-only compatibility
surface.

## Implementation plan

1. Model Task rework inputs with the existing preview and routine mutation
   execute envelopes, including the reason recorded by `task.rework_started`.
2. Forward only `task.rework` through the fixed command preview/execute paths;
   preserve Workspace revision, Actor attribution, idempotency, warning
   acknowledgement, and proceed reason.
3. Register the two exact tool names in compact and full profiles as routine
   Operator tools. Keep all prior full-profile names and remove only
   `baley_workspace_get` from compact.
4. Pin exact names, counts, schema bytes, annotations, descriptions, required
   fields, HTTP paths, arguments, and envelopes in focused tests and the
   literal contract.
5. Update operating documentation and Baley skill guidance, then run focused
   and full repository verification and review the complete diff.

## Safety boundaries

- Do not change `task.rework` lifecycle, capability, approval, Event, revision,
  warning, or idempotency semantics.
- Do not add typed `task.block` or `task.unblock` tools.
- Do not install or replace the operating MCP, touch operating data, change
  firewall state, restart services, commit, push, merge, or deploy.
- Keep Task Records pending; the coordinator owns Baley registration and Task
  state mutations.
