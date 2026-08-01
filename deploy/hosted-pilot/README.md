# Hosted Pilot deployment artifacts

These artifacts implement the accepted initial layout:

```text
lucy: Viewer + loopback baley-server
devhub: shared PostgreSQL on the Tailscale address
```

They are production templates, not commands that may be applied blindly. Existing
services on either host remain outside this deployment.

## 1. Build an immutable release

From a clean Windows checkout:

```powershell
.\scripts\build-hosted-pilot-release.ps1 -Version 0.1.0
```

The output under `.tmp\hosted-pilot-releases` contains the Linux amd64 server,
Viewer, migrations, build metadata, and `SHA256SUMS`. A dirty build requires the
explicit `-AllowDirty` flag and is not a production candidate.

## 2. Prepare devhub

Read `devhub/README.md`. The Compose template binds PostgreSQL to the exact
Tailscale IPv4 address supplied through the external environment file. It does
not publish on the AWS private or public interface.

The PostgreSQL service is shared infrastructure. Baley gets its own database and
separate application, migration, and backup roles. Other applications must use
different databases and roles.

## 3. Prepare lucy

Read `lucy/README.md`. Copy a verified release into `/srv/baley/releases`, install
the systemd and Caddy templates, run migration explicitly, then activate the
release. The API listens only on `127.0.0.1:8080`; Caddy listens on
`127.0.0.1:8081` for a future Cloudflare Tunnel.

The existing Lucy English container, 443 listener, 3001 listener, and certificate
mount are not part of these files and must not be changed during Baley staging.

## 4. Human approval boundaries

Approval is required immediately before:

- starting or changing the devhub PostgreSQL listener;
- enrolling lucy into the tailnet or changing Tailscale policy;
- installing or starting the lucy Baley services;
- restoring or replacing a non-disposable database;
- creating the public hostname/Tunnel or inviting Pilot users.

Repository builds, tests, checksum verification, and isolated restore verification
do not cross those boundaries.
