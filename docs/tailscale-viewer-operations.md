# Tailscale Viewer operations

Baley's local Docker deployment can be viewed from any authorized Tailnet device
without exposing the API, PostgreSQL, or Docker ports publicly. The Viewer and
its same-origin `/api` proxy remain bound to loopback; Tailscale Serve is the
only Tailnet ingress.

## Canonical URL

Use the HTTPS MagicDNS URL shown by `tailscale serve status`, for example:

```text
https://jazzcake-home.tail87e929.ts.net/
```

Do not use `http://127.0.0.1:5174` for normal operation. It is a local
diagnostic endpoint only. HTTPS provides browser secure-context features such
as the native Clipboard API, in addition to Tailnet transport encryption.

## One-time Tailnet setup

An Owner enables **DNS → HTTPS Certificates → Enable HTTPS** in the Tailscale
admin console. MagicDNS must be enabled. Tailscale publishes the machine and
tailnet certificate name in the public certificate-transparency ledger; access
to the service remains restricted by Tailnet policy.

## Start or restore the ingress

Start the local stack first:

```powershell
docker compose up -d --build
```

Then configure persistent HTTPS Serve:

```powershell
.\scripts\enable-tailscale-viewer.ps1
```

The script proxies HTTPS port 443 to `http://127.0.0.1:5174`, disables the
Tailnet HTTP listener on port 80, and prints the canonical URL. `--bg` makes
the Serve configuration survive a Tailscale or host restart.

## Verify

```powershell
docker compose ps
tailscale serve status
```

Confirm the Viewer loads at the HTTPS URL, login works, and
`/api/readyz` returns a ready response through the same origin. The API remains
loopback-only on port 8080 and PostgreSQL remains loopback-only on port 54329.
