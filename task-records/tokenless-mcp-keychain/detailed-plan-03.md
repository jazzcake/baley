---
baley_record: 1
record_id: "eb39a3ea-73fd-4816-aa74-5b76e92d6363"
task_id: 149
task_key: "tokenless-mcp-keychain"
record_type: detailed-plan
run_id: "18b2c250-e49c-44ff-bf17-e079d3f7b9a2"
created_at: "2026-08-28T00:00:00Z"
created_by: "codex"
supersedes: null
---

# #149 implementation handoff plan: tokenless MCP validation and closeout

## Current baseline

The implementation branch already contains the tokenless stdio MCP client,
OS-keychain-backed device secret handling, legacy credential-store migration,
revoke-safe diagnostics, and signed-in automatic gateway linking. Commit
`b2be80a` additionally removes the one-time gateway approval button for
authenticated Workspace operators. The local Codex registration contains only
`BALEY_SERVER_URL` and `BALEY_MCP_CREDENTIAL_STORE`; it must not contain a
gateway token or an Authorization header.

## Implementation and verification work

1. Inspect the current credential, keychain, migration, transport, and
configuration code against #149. Close any gaps rather than reimplementing
already-shipped behavior. Preserve tokenless stdio as the Desktop/CLI path and
keep every optional HTTP listener loopback-only.
2. Exercise a fresh Codex Desktop/CLI process with no
`BALEY_MCP_GATEWAY_TOKEN`. Verify diagnosis reports a keychain-backed store and
that an authenticated Workspace member can query Baley after a restart.
3. Verify negative paths without leaking secrets: logout, membership removal,
gateway revoke, and suspected credential replacement must make cached local
credentials unusable immediately. Exercise legacy migration/rollback only with
test fixtures or a disposable store; no production or user secret is copied
into source, docs, config, or logs.
4. Run focused Go tests plus `go test ./...`, `go vet ./...`, frontend tests,
and production build. Deploy the affected services and perform a deployed
tokenless MCP smoke test. Record all commands and results in the completion
report.
5. Commit and push the completed work. Produce an independent-agent review and
review response before reporting #149 implemented. Human confirmation remains
out of scope.

## Non-negotiable boundaries

- OAuth may automate MCP connection only. It must not create or bypass Task
  confirm, Gate condition changes, Gate pass, or Gate Task pass authority.
- Never write a gateway token to Codex `config.toml`, an Authorization header,
  plain local storage, repository files, test fixtures, screenshots, or chat.
- A keychain failure is actionable; it must not silently fall back to a
  plaintext or environment-derived device secret.

