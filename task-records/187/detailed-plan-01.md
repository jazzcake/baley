---
baley_record: 1
record_id: "4aee9f57-a73e-42d7-9456-27d4e507ac73"
task_id: 187
record_type: detailed-plan
run_id: "f9229bab-1414-45d1-92b1-4ca009e2db9b"
created_at: "2026-09-10T18:50:00+09:00"
created_by: "codex-worker-term_7703ed58"
status: active
---

# Task #187 provider-authoritative remote verification plan

## Problem and trust boundary

Seven superseding Task Records for #184 and #186 point at commit
`a1834a53677443377970acbcf0a497302d5987c8`, but remain
`committed_unverified` and have no matching CommitReference. A caller-provided
claim about a push, ref tip, blob, or content digest is not verification.

## Implementation

Add the supported `commit.verify_remote` Operator command to the literal command
contract and compact generic bridge catalog. The command accepts only a
CommitReference ID and a full branch ref; the service obtains the remote URL
from the registered Repository, fetches the branch with interactive credentials
disabled, proves the exact commit is reachable, resolves every matching
`commit:path`, compares each fetched blob ID, hashes the fetched bytes with
SHA-256, and compares each registered content hash.

After every check succeeds, update the CommitReference and all matching records
in one revision-guarded transaction. Preserve an immutable
`commit.remote_verified` Event plus one `record.remote_verified` Event per file,
including remote/ref tip, commit, path, blob, content digest, verifier, command,
actor, and revision provenance. Provider failure, timeout, mismatch, stale
revision, mixed state, or row-count conflict must leave the states unchanged.

## Verification and rollout

Cover domain transition invariants, real Git fetch/ref/blob/content behavior,
application failure and replay behavior, HTTP execution, PostgreSQL atomic
persistence, audit classification, literal contracts, and compact catalog
counts. Run full Go test/vet against a disposable schema-28 PostgreSQL, Viewer
tests/build only if affected, MCP catalog and UTF-8 checks, build reviewed
executables under `C:\dev-bin\baley`, deploy without firewall or Tailscale
changes, then attach and remotely verify the seven records using only supported
commands. Leave #184, #185, #186, and #187 unconfirmed.
