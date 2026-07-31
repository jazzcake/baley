---
baley_record: 1
record_id: "4b124c86-4a23-4d26-b728-855241743bf8"
task_id: 132
task_key: "account-workspace-viewer"
record_type: detailed-plan
run_id: "aebb6da4-37d6-45c2-a5d0-91327f1e6874"
created_at: "2026-07-28T00:34:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Account-bound Workspace Viewer detailed plan

Task #132 replaces the fixed development Workspace with an authenticated account
and Workspace selection flow.

1. Add credentialed typed clients for session, Workspace membership, graph, and
   member administration.
2. Add session bootstrap, login/logout, Workspace chooser, Workspace-scoped routes,
   and account/role controls.
3. Pass the route Workspace ID into every graph request and remove the environment
   Workspace ID as the normal authority.
4. On Workspace switch, abort old polling, increment a request generation, and
   reset graph, selection, focus, backlog, layout, and viewport state.
5. Add an Owner-only member administration surface, while retaining server-side
   authorization as the actual boundary.
6. Add development-only structured traces across the user event, target, auth
   state, route, request generation, committed graph, and rendered DOM.
7. Verify login/session states, race-safe switching, role presentation, last-Owner
   conflict handling, accessibility, polling recovery, and production build.
