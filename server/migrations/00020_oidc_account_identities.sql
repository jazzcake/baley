-- +goose Up
-- External identities are deliberately keyed by the OIDC issuer and subject.
-- Email is an optional presentation claim and is never an account identifier.
CREATE TABLE account_external_identities (
  id uuid PRIMARY KEY,
  account_id uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  provider_id text NOT NULL CHECK (length(provider_id) BETWEEN 1 AND 80),
  issuer text NOT NULL CHECK (length(issuer) BETWEEN 1 AND 2048),
  subject text NOT NULL CHECK (length(subject) BETWEEN 1 AND 1024),
  created_at timestamptz NOT NULL DEFAULT now(),
  last_authenticated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (issuer, subject)
);
CREATE INDEX account_external_identities_account ON account_external_identities(account_id);

-- The PKCE verifier is encrypted at rest by the server state key. State,
-- nonce and device binding remain one-way digests.
CREATE TABLE oidc_authorization_flows (
  state_hash bytea PRIMARY KEY CHECK (octet_length(state_hash) = 32),
  provider_id text NOT NULL CHECK (length(provider_id) BETWEEN 1 AND 80),
  nonce_hash bytea NOT NULL CHECK (octet_length(nonce_hash) = 32),
  binding_hash bytea NOT NULL CHECK (octet_length(binding_hash) = 32),
  verifier_ciphertext bytea NOT NULL CHECK (octet_length(verifier_ciphertext) BETWEEN 29 AND 1024),
  intent text NOT NULL CHECK (intent IN ('login','link')),
  link_account_id uuid REFERENCES accounts(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  consumed_at timestamptz,
  CHECK ((intent = 'login' AND link_account_id IS NULL) OR (intent = 'link' AND link_account_id IS NOT NULL))
);
CREATE INDEX oidc_authorization_flows_expiry ON oidc_authorization_flows(expires_at) WHERE consumed_at IS NULL;

-- +goose Down
DROP TABLE oidc_authorization_flows;
DROP TABLE account_external_identities;
