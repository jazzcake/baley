---
baley_record: 1
record_id: "8c229456-4972-437c-a85d-437b4e76d4e7"
task_id: 151
task_key: "workspace-mcp-context"
record_type: detailed-plan
run_id: "e03a6a4f-ca08-48a6-97ca-568581df809f"
created_at: "2026-08-28T00:00:00Z"
created_by: "codex"
registration_state: pending
---

# #151 compact MCP Workspace context plan

## Default read contract

Add a new read-only `baley_workspace_context` MCP tool and matching HTTP API
route. It is the first-read path for a Workspace: completed Phases are omitted;
every remaining Phase is represented only by its ID, name, state, position,
and compact per-Lane Task-status counts. The response includes Workspace
revision and a `fullGraphAvailable` signal, but no Task title, description,
dependency, record, Run, or completed-Phase payload.

The existing `baley_workspace_graph` endpoint remains the explicit complete
graph opt-in for Viewer compatibility and deliberate deep investigation. A
caller expands work only with existing `baley_task_get`, `baley_lane_brief`, or
the planned phase-scoped paged Task route; initial context never silently
serializes every Task.

## Implementation slices

1. Define an application-level compact projection that derives non-completed
   Phases and deterministic Lane/status counts from a single authorized
   snapshot. Preserve Workspace revision and existing auth middleware.
2. Add `GET /v1/workspaces/{workspaceId}/context` and the MCP
   `baley_workspace_context` read tool. Keep `/graph` unchanged and describe it
   as the explicit full-graph path.
3. Add a phase-scoped, cursor-paged Task listing endpoint/tool so a caller can
   expand one named Phase without loading unrelated completed or active work.
4. Test auth protection, completed-Phase exclusion, count accuracy,
   deterministic ordering, cursor bounds, and MCP tool registration. Add a
   benchmark fixture at 100, 1,000, and 10,000 Tasks measuring encoded bytes,
   p95 latency, and allocations for compact context versus full graph.
5. Update MCP operational documentation and verify API, MCP, Viewer
   compatibility, and deployed Pilot behavior. No read model changes Task/Gate
   mutation or human-only approval authority.

## Compatibility and risk controls

- Full graph consumers retain their current endpoint/tool; compatibility is
  preserved by addition, not by weakening the graph payload.
- Counts are generated after the same snapshot and authorization boundary as
  today, so they cannot expose hidden Task details.
- Cursor expansion returns only the requested non-completed Phase and has a
  bounded default/maximum page size to protect the server and Codex context.
