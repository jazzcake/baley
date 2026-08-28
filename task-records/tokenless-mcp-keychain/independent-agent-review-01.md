---
baley_record: 1
record_id: "f64b3c4d-8309-4166-ab84-390b2f05ad60"
task_id: 149
task_key: "tokenless-mcp-keychain"
record_type: independent-agent-review
run_id: "9f62d55f-15fe-4ac8-bf14-2f4cfbdc74f1"
created_at: "2026-08-28T12:00:00Z"
created_by: "codex"
supersedes: null
---

# #149 independent security review

## Scope

An independent Agent reviewed the tokenless stdio MCP client, keychain and
credential-store handling, the local scripts and handoff material, gateway
revocation/membership behavior, and the Task/Gate authority boundary.

## Blocking findings

1. A keychain payload could retain an `AgentToken`. A new MCP process returned
   that value before renewing its gateway, so revocation was detected only when
   a later API request failed.
2. Retired local helper material still implemented and documented writing an
   Agent credential into a plaintext environment file.

## Confirmed boundaries

Server-side gateway replacement, revocation, and membership removal revoke
derived Agent credentials. OAuth connection remains separate from human-only
Task confirmation, Gate condition changes, Gate Task pass, and Gate pass.

## Required response

Make Agent credentials process-memory-only; scrub historic persisted values
before use; remove the plaintext helper path and its guidance; add regression
coverage for fresh-process renewal, revocation, keychain storage, and legacy
migration; then repeat the live tokenless smoke test.
