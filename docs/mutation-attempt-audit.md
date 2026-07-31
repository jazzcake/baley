# Mutation attempt audit

Baley records every command execution attempt in PostgreSQL table
`mutation_attempts`. This is an operational audit trail that complements, but
does not replace, the authoritative `commands` and `events` history.

Each row belongs to exactly one `workspace_id` and records the command name,
outcome (`succeeded`, `rejected`, `failed`, or `idempotent`), target when it can
be inferred, actor IDs, Command/Event links, revisions, diagnostic codes, and
duration. Raw arguments, idempotency keys, approval text, SQL text, lease
tokens, and error messages are not stored. Request arguments and idempotency
keys are represented only by SHA-256 digests.

The command-service row is inserted in a separate short transaction after the
domain transaction finishes. Therefore a rejected command or a rolled-back
write still leaves an attempt row. A transaction-local marker prevents the
database trigger from duplicating application writes. Direct INSERT, UPDATE,
or DELETE operations against `tasks` are logged by a database trigger in the
same transaction.

`mutation_attempts` is append-only: UPDATE, DELETE, and TRUNCATE are rejected.
Its lifecycle should therefore be handled with PostgreSQL backup, partitioning,
or an explicit future retention migration rather than ad-hoc deletion.

Read one workspace only:

```http
GET /v1/workspaces/{workspaceId}/mutation-attempts
    ?outcome=rejected
    &commandName=task.create
    &after=2026-07-26T08:00:00Z
    &limit=50
```

The equivalent typed MCP read tool is `baley_mutation_attempt_list`.
