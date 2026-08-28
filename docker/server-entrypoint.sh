#!/bin/sh
set -eu

secret_file=/var/lib/baley/lease_token_secret
if [ ! -s "$secret_file" ]; then
  umask 077
  if [ -s /legacy-secrets/lease_token_secret ]; then
    cp /legacy-secrets/lease_token_secret "$secret_file"
  else
    head -c 32 /dev/urandom | base64 > "$secret_file"
  fi
fi

export BALEY_LEASE_TOKEN_SECRET_FILE="$secret_file"

# OIDC state encryption needs a persistent server-only key so a normal API
# restart does not invalidate an in-flight callback. It is independent of the
# MCP lease signer and is never copied into client configuration.
oidc_state_file=/var/lib/baley/oidc_state_secret
if [ ! -s "$oidc_state_file" ]; then
  umask 077
  head -c 32 /dev/urandom | base64 > "$oidc_state_file"
fi
export BALEY_OIDC_STATE_SECRET_FILE="$oidc_state_file"
/app/baley-server migrate up
exec /app/baley-server serve
