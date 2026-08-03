---
name: baley-adopt-project
description: Adopt an existing Git project into a Baley Workspace for a controlled pilot. Use when an Operator must bind a repository, install Task Record templates, exercise Backlog/Task/Run/evidence recovery, or collect a PilotMeasurement without crossing human approval boundaries.
---

# Baley Adopt Project

Use this skill with `baley-manage-work`; that skill remains authoritative for
Run, Record, Task confirmation, and Gate approval rules.

## Preconditions

1. Confirm the repository root and read `baley.yaml` when it exists.
2. Confirm the Baley server is healthy and the signed-in account can see the
   target Workspace.
3. If the Workspace does not exist, an authenticated human creates it from the
   Workspace dropdown. Creation atomically binds that account as Owner and
   installs the Intake Phase, Adoption Lane, counters, and human-required
   acceptance baseline. Do not invent membership by direct SQL.
4. Verify that the account is an Owner or Participant and that the Operator
   credential is scoped to the exact Workspace.
   When the first typed MCP read returns `workspace_connection_required`, give
   the Owner its approval URL and retry that same read after approval. Do not
   create a per-Workspace env file, copy a token, register another MCP server,
   or request a new thread.
5. Never put passwords, agent tokens, Run lease tokens, or
   credential-bearing URLs in config, Task Records, commands, or logs.

Read [references/operations.md](references/operations.md) before mutating
Baley or the project. For a full Enablement acceptance run, also read
[references/acceptance-scenario.md](references/acceptance-scenario.md).

## Adopt the repository

1. Fresh-read the Workspace and repository list.
2. Preview/register the repository as the record repository with a
   repository-relative Task Record root.
3. Run the installed `baley-project-init` CLI in preview mode for `baley.yaml`,
   `.rgignore`, `.baley-init-state.json`, README, and templates.
4. Apply only `create` and verified `merge` actions. Stop on `conflict`; never
   overwrite an existing user file.
5. Rerun the planner. Completion requires every desired file to be `keep`.
6. Verify the config server, Workspace ID, repository ID, record repository
   ID, and Task Record root against a fresh server read.
7. Remove `.baley-init-state.json` after the server binding and all files are
   verified. It is retry state, not a durable project artifact.

`project.bootstrap` is not a public execution transport in the current
runtime. The supported pilot path is Owner-created Workspace followed by typed
`repository.register` and `baley-project-init` with
`bootstrapCompleted: true`. Do not simulate bootstrap with direct database
writes.

## Operate the pilot

- Create ideas in the lane Backlog without assuming a Phase.
- Promote only when a Phase and dependency intent are known.
- Start a Run before planning, implementation, review, or reporting.
- On session loss, fresh-read Runs and the lane brief before acting.
- Treat the lane brief as read-only. Never repair a mismatch from a read path.
- Register exact repository-relative evidence and preserve Git/Record
  mismatches until an explicit correction produces a new Event or Record.
- Allow delegated auto-confirm only when the bound evidence profile passes.
- Keep human-required Task confirmation and Gate/Lane/Workspace decisions
  pending for an authenticated human approval.

## Measure

Copy `_templates/pilot-measurement.md`, fill its JSON payload, resolve this
installed Skill's directory, and run its validator:

```powershell
python <skill-directory>/scripts/validate_pilot_measurement.py <measurement.md>
```

Register the validated file with Record type `pilot-measurement`. Measurements
are append-only; corrections create a new Record and reference correction
Event IDs. Never edit an already registered sample in place.

## Completion

Report the exact Workspace revision, Runs, Records, Git evidence, validation
commands, mismatch/correction evidence, and remaining human decisions. A
successful technical run does not authorize Task confirmation or Gate passage.
