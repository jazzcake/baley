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
/app/baley-server migrate up
exec /app/baley-server serve
