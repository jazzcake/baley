---
baley_record: 1
record_id: "db66e02d-410f-46c0-bcca-2991abe652a5"
task_id: 151
task_key: "workspace-mcp-context"
record_type: detailed-plan
run_id: "0ca498c7-cfc1-4954-b77e-b1922f6b46d4"
created_at: "2026-08-28T00:00:00Z"
created_by: "codex"
supersedes: null
---

# #151 implementation handoff plan: compact Workspace MCP context

## Current baseline

`baley_workspace_context` and phase-scoped paged Task expansion are already
available in the live Pilot. The initial context returns only active
non-completed Phase metadata and Lane/status counts; the caller explicitly
expands `multi-user-operations` to obtain its Task list. Full graph remains an
opt-in compatibility path.

## Implementation and verification work

1. Inspect the actual application projection, HTTP route, MCP tool registration
and consumers. Compare their behavior to #151 and identify only remaining
gaps: revision/delta semantics, page bounds, authorization filtering,
deterministic ordering, error contracts, and documentation.
2. Add or tighten benchmarks at 100, 1,000, and 10,000 Tasks. Measure compact
context versus full graph encoded bytes, p95 latency, allocations, and the
estimated Codex context payload. Set evidence-backed regression thresholds
where the repository test style supports them; do not fabricate performance
numbers.
3. Add regression coverage for completed-Phase exclusion, active Phase summary
accuracy, no Task-description leakage in default context, cursor isolation and
bounds, stale revision/delta behavior, existing Viewer/full-graph
compatibility, and MCP tool schemas.
4. Run focused Go tests and benchmarks plus `go test ./...`, `go vet ./...`,
frontend tests, and production build. Deploy and verify that another project
can obtain the compact context then expand a named Phase without loading the
whole Workspace.
5. Commit and push. Produce an independent-agent review and review response,
then create a completion report before reporting #151 implemented. Human
confirmation is not part of this work.

## Compatibility and security boundaries

- Default compact reads must apply the same Workspace authorization boundary as
  existing reads and expose no hidden Task identity or details through counts.
- Keep existing full-graph endpoints/tools available as explicit opt-in. Do not
  break Viewer consumers merely to make Codex reads smaller.
- This is a read-model/performance change only: Task and Gate human-only
  decisions, audit behavior, and write contracts must remain unchanged.

