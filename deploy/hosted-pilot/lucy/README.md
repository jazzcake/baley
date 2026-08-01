# Lucy Web/Application deployment

This host runs the Viewer and loopback Baley API. It does not build artifacts and
does not own PostgreSQL.

## Prerequisites

- Lucy is enrolled in the same tailnet as devhub.
- Caddy is installed at `/usr/bin/caddy`.
- A release directory was transferred to `/srv/baley/releases` and its
  `SHA256SUMS` passed.
- `/etc/baley/secrets/database_url` contains the Baley application URL using the
  devhub Tailscale hostname/address and `sslmode=disable`.
- `/etc/baley/secrets/migration_database_url` contains the separate migration
  role URL for the same database.
- `/etc/baley/secrets/lease_token_secret` contains the stable lease secret.
- All secret files are owned by root and readable only by root. systemd passes
  them to the service as credentials.

Do not put those values in `baley.env`.

After transfer, mark only the server and activation script executable. Changing
file modes does not change release content checksums:

```bash
sudo chmod 0755 /srv/baley/releases/RELEASE/baley-server
sudo chmod 0755 /usr/local/sbin/baley-activate-release
```

## One-time installation outline

These are deliberate host mutations and require Owner approval before execution.

1. Create the `baley` system user, `/srv/baley/releases`, `/etc/baley/secrets`,
   and `/etc/baley` with restrictive ownership.
2. Copy `baley.service`, `baley-migrate.service`, `baley-web.service`, and
   `Caddyfile` to `/etc` locations. Install units and Caddy configuration as
   root-owned mode `0644` files.
3. Copy `baley.env.example` to `/etc/baley/baley.env` and replace the example
   production origin; keep it root-owned with mode `0600`.
4. Install `activate-release.sh` under `/usr/local/sbin`.
5. Verify unit and Caddy syntax before enabling anything:

```bash
sudo systemd-analyze verify /etc/systemd/system/baley.service
sudo systemd-analyze verify /etc/systemd/system/baley-migrate.service
sudo systemd-analyze verify /etc/systemd/system/baley-web.service
sudo caddy validate --config /etc/baley/Caddyfile
```

## Migration and activation

Create and verify a fresh devhub backup before activation. Migration is explicit
inside the guarded activation script via `baley-migrate.service`; it is never part
of regular API service startup.

```bash
sudo /usr/local/sbin/baley-activate-release baley-VERSION-linux-amd64
```

The script verifies checksums, switches the `current` symlink, starts the one-shot
migration unit, restarts the two Baley services, waits for `/readyz`, and restores
the previous release link on failure. Database migrations must remain additive
and backward compatible because application rollback does not downgrade schema.

Cloudflare Tunnel/public DNS is a later approval boundary. Until then, test Caddy
only through `127.0.0.1:8081` on lucy.
