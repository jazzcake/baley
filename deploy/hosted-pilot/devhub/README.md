# devhub shared PostgreSQL

This template provides PostgreSQL to authorized tailnet clients. It binds the
host port to the exact devhub Tailscale IPv4 address and never to `0.0.0.0` or the
AWS private address.

## One-time preparation

These steps mutate devhub and require Owner approval immediately before execution.

1. Copy this directory to an isolated deployment path on devhub.
2. Copy `postgres.env.example` to `postgres.env` and replace the example address
   with `tailscale ip -4` output. Keep this file owned by root with mode `0600`.
3. Store a strong admin password at the external path configured by
   `POSTGRES_ADMIN_PASSWORD_FILE`; use owner root and mode `0600`.
4. Validate before starting:

```bash
./validate-config.sh ./postgres.env
docker-compose --env-file ./postgres.env -f ./compose.yml config
```

5. Install `devhub-postgres.service` only after its fixed paths match the chosen
   deployment and environment-file locations. The unit orders startup after both
   Docker and Tailscale so the Tailscale-only host bind cannot race interface
   creation. Compose itself deliberately uses `restart: "no"`; systemd owns restart.
6. Verify the unit before enabling it:

```bash
sudo systemd-analyze verify /etc/systemd/system/devhub-postgres.service
```

7. Review Tailscale policy for the PostgreSQL port. Tailnet reachability does not
   replace PostgreSQL role/password authentication.
8. Start only after confirming the bind address from rendered Compose output.

`POSTGRES_INITDB_ARGS` applies only when PostgreSQL initializes an empty data
volume. If an existing volume is ever reused, inspect its `pg_hba.conf` instead
of assuming the Compose value changed the existing authentication rules.

## Create Baley identities

Use PostgreSQL's interactive password prompt so application passwords do not enter
shell history or process arguments:

```bash
docker-compose --env-file ./postgres.env -f ./compose.yml exec postgres \
  createuser --login --no-createdb --no-createrole --no-superuser --pwprompt baley_migrator
docker-compose --env-file ./postgres.env -f ./compose.yml exec postgres \
  createdb --owner=baley_migrator baley
docker-compose --env-file ./postgres.env -f ./compose.yml exec postgres \
  createuser --login --no-createdb --no-createrole --no-superuser --pwprompt baley_app
docker-compose --env-file ./postgres.env -f ./compose.yml exec postgres \
  createuser --login --no-createdb --no-createrole --no-superuser --pwprompt baley_backup
```

Before migration, grant future application-object access as the database owner:

```bash
docker-compose --env-file ./postgres.env -f ./compose.yml exec postgres \
  psql -U baley_migrator -d baley -v ON_ERROR_STOP=1 -c \
  'GRANT CONNECT ON DATABASE baley TO baley_app,baley_backup; GRANT USAGE ON SCHEMA public TO baley_app,baley_backup; ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT,INSERT,UPDATE,DELETE ON TABLES TO baley_app; ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE,SELECT,UPDATE ON SEQUENCES TO baley_app; ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO baley_backup; ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON SEQUENCES TO baley_backup;'
```

After the first migration, also grant the application role access to the objects
that now exist. The default privileges cover later migrations:

```bash
docker-compose --env-file ./postgres.env -f ./compose.yml exec postgres \
  psql -U baley_migrator -d baley -v ON_ERROR_STOP=1 -c \
  'GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO baley_app; GRANT USAGE,SELECT,UPDATE ON ALL SEQUENCES IN SCHEMA public TO baley_app; GRANT SELECT ON ALL TABLES IN SCHEMA public TO baley_backup; GRANT SELECT ON ALL SEQUENCES IN SCHEMA public TO baley_backup;'
```

Lucy uses `baley_app` for the running API and `baley_migrator` only in the
one-shot migration service. Database dumps use `baley_backup`; only the cluster
globals export uses the PostgreSQL administrator. Do not reuse another
application's role in any Baley URL.

The Lucy database URL uses the devhub Tailscale hostname/address and
`sslmode=disable`; transport encryption is supplied by Tailscale.

## Backup and isolated restore verification

```bash
sudo BALEY_BACKUP_ROOT=/srv/devhub-postgres/backups ./backup-baley.sh
sudo ./verify-backup.sh /srv/devhub-postgres/backups/baley-UTC_TIMESTAMP
```

The backup script writes a custom-format database dump, cluster globals, and
SHA-256 checksums. The verification script restores into a uniquely named
temporary database, checks the Workspace table, and drops only that temporary
database. It never replaces the live database.

Copy encrypted backups to storage outside devhub. A backup remaining only on the
database host is not a recovery copy.
