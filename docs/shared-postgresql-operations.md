# Shared PostgreSQL operation

Baley uses the shared local PostgreSQL service rather than its former
project-local database container.

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

## Recovery

The project-local PostgreSQL container and volume were retired after verified
logical migration. Recovery now uses the shared PostgreSQL service's backup and
restore process; do not recreate a project-local database as an ad-hoc fallback.
