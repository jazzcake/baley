# Shared PostgreSQL operation

Baley uses the shared local PostgreSQL service rather than its former
project-local database container.

The infrastructure SSOT is maintained in
[`local-dev-infra`](../../local-dev-infra/README.md). Its Git-ignored `.env`
holds only the shared bootstrap credentials; Baley owns its separate `baley`
database and `baley_app` application role.

## Active connection

- Shared container: `local-dev-postgres`
- Docker network: external `local-dev`
- Baley database: `baley`
- Application role: `baley_app`
- Container hostname: `local-dev-postgres:5432`

`local-dev-postgres` is intentional. It is the shared container's unique name
and avoids ambiguity with generic service aliases on Docker networks.

The Docker Compose configuration reads `BALEY_DB_PASSWORD` from the invoking
user environment. Store it only as a local user secret; never add it to a
tracked file, an MCP configuration, a command transcript, or a URL.

For a new terminal after a local setup, set the process variable from the
Windows User environment before starting Compose:

```powershell
$env:BALEY_DB_PASSWORD = [Environment]::GetEnvironmentVariable('BALEY_DB_PASSWORD', 'User')
docker compose up -d api mcp viewer
```

## Migration verification

The initial cutover used `pg_dump`/`pg_restore` in custom format with
`--no-owner --no-privileges`. Verify a future migration before cutover by
comparing the applied `goose_db_version` and public-table row counts between
the source and target. Then verify:

```powershell
Invoke-WebRequest http://127.0.0.1:8080/readyz
Invoke-WebRequest http://127.0.0.1:5174/
```

Also make one authenticated MCP read, such as `baley_workspace_get`, after
the API and MCP services are healthy.

## Cutover log — 2026-08-14

- Migrated the former project-local `baley` database from `baley-postgres-1`
  to the shared `local-dev-postgres` PostgreSQL 17.5 instance.
- Created the database-local application role `baley_app` without cluster
  administration privileges.
- Verified that every public-table row count and the applied Goose migration
  version (`18:true`) matched between source and target.
- Recreated `api`, `mcp`, and `viewer` against the shared database; API
  readiness, Viewer shell delivery, authenticated MCP workspace read, and a
  Docker service restart all passed.
- After verification, removed the stopped `baley-postgres-1` container and
  its `baley_baley-postgres` volume. The shared database is now the sole
  Baley PostgreSQL runtime.

## Recovery

The project-local PostgreSQL container and volume were retired after verified
logical migration. Recovery now uses the shared PostgreSQL service's backup and
restore process; do not recreate a project-local database as an ad-hoc fallback.
