---
baley_record: 1
record_id: "b6786f31-dfee-4c09-a23a-9423a80e8469"
task_id: 187
record_type: completion-report
run_id: "d04166ef-587c-4da7-a735-cdf8659bd924"
created_at: "2026-09-10T20:22:56+09:00"
created_by: "codex-worker-term_7703ed58"
status: complete
---

# Task #187 completion report

## Outcome

Implemented and deployed provider-authoritative remote Git verification. The
server fetches the configured Repository remote and full branch ref, proves the
exact commit is reachable, resolves every matching `commit:path`, compares Git
blob IDs, hashes fetched blob bytes, and changes the CommitReference and Records
atomically only after all evidence matches. Remote-ref binding is reconstructed
from database-enforced append-only Events; zero records, a different replay ref,
provider failure, stale revision, evidence mismatch, and row-count races fail
closed. The API image runs Git below a `tini` PID-1 reaper.

Reviewed implementation commits are `fb18bd7d98477b3ec133485421ad9a2d142c8c13`,
`e613a7de55dcdd3a5d81aa78db93ae57d06bcb73`, and deployment commit
`5a814f3c444e709dfdecab63c13fc0e614a6f9bb`. The independent-review Record was
added in evidence-only commit `4f260c1a3094c8e457a9c85964434124c7eedb7a`.
All were pushed to `origin/jazzcake/task-journal-finalize`.

## Baley provenance

- Task #187: `7980fc93-37d9-44d4-8650-48369bba6af1`; creation command
  `4eddae4c-10ff-4117-8f23-4614287a8409`, Event/Journal
  `14fdf183-243d-475a-b346-aa126e3509d0`.
- Recovered interrupted implementation Run:
  `2a8850de-67d3-4001-9b30-2f62d4400bca`.
- Succeeded implementation Run:
  `f9229bab-1414-45d1-92b1-4ca009e2db9b`; terminal command
  `0674018b-494a-4ca0-a2ae-0bcc0187c128`, Event
  `b5f395c8-de52-4a2c-bc77-249e72629cb3`.
- Completion-reporting Run: `d04166ef-587c-4da7-a735-cdf8659bd924`;
  start command `f60565b1-4d88-4548-b285-a8aa80af2616`, Event/Journal
  `66957a4d-b1ba-4011-ad3a-46153e163e60`.
- Detailed plan Record: `4aee9f57-a73e-42d7-9456-27d4e507ac73`.
- Independent review Record: `af622fd7-68aa-4ef0-bfc9-c6f5a1afe01e`;
  reviewer `term_a785ee93-72d4-4fc2-82d4-fa91111da738`, Orca task
  `task_3a421f6d5bef`, verdict zero blockers and 0/0/0 findings.

## Live provider verification

CommitReference `39174f34-438f-4364-ac31-a4f35010598b` binds commit
`a1834a53677443377970acbcf0a497302d5987c8`; attach command
`5ff451ba-8e4b-4d80-99ef-5893fd68677b`, Event
`f19a47f3-91f2-4ab8-947a-cea785414aca`. Verification command
`4d126f9e-f17b-4e29-9e94-6ee378bf9f35` fetched
`https://github.com/jazzcake/baley`, ref
`refs/heads/jazzcake/task-journal-finalize`, tip
`4f260c1a3094c8e457a9c85964434124c7eedb7a`, using `git-fetch-v1` at
`2026-09-10T11:14:45.655882545Z`. It advanced Workspace revision 1463 to 1464
and emitted:

- commit Event `8a8d0e9f-e1fe-42ab-9f63-16bb21256649`;
- record Events `ebc365e7-a3c6-47a2-9ec5-6a70aae3483e`,
  `9fd268a8-b332-44a0-982e-c80318cb4f27`,
  `f88c1130-899f-431b-807d-5bd790a92db0`,
  `b353a62a-e96f-4eaf-9b2e-883f440d9358`,
  `e25253dc-afee-4d2f-9199-6b2410bb8805`,
  `438be400-f65b-485a-9dc7-055b083f1412`, and
  `40b0696b-0f11-4e6a-8597-6bdba089dd30`.

The seven verified Records are `8ae81c03-ba5e-44d8-a0df-88d7c2fdeb21`,
`ae58d32f-d091-4ce1-9b44-eec1fc982c15`,
`d079eefd-48ea-40ce-8b4c-44e503de6781`,
`649a73e2-3cab-408a-b112-d0a0a5006a98`,
`f7460a1c-e9cb-4de6-9a15-99e6735b34db`,
`14360081-7302-4966-b67d-85ef0bb482b5`, and
`11fa0c87-7522-4521-8897-67a8b0f52d47`. State replay command
`13af1244-567c-45a1-987c-9edb8220c1ae` was idempotent at revision 1464 with
zero Events; a `refs/heads/main` replay returned
`commit_remote_unverified`/HTTP 422 without revision change.

## Deployment and verification

- API image `sha256:9ac4f8be3a60929b67e259faaa27007405a32542c5cea2188fb75c31e7716fbd`
  is labelled revision `5a814f3...`, schema 29, and is healthy. MCP runs
  `C:\dev-bin\baley\releases\5a814f3c444e\baley-mcp.exe`; its SHA-256 is
  `089AC705F6E6AF96E8AA8B331BFFB3AA9EDA734F30CCB543058E22F2FC0DA141`.
  The API executable SHA-256 is
  `55AC3FD7BC11935446EC8027DD864F2F6A134D6AF2F1E8D18571432B80EC7ADB`.
- Pre-migration schema-28 dump SHA-256 is
  `AA6AA947A1DD16480FA7EC51FC0A247746035902DC0E31A8922DCA42A731A2C8`.
  Live schema 29 has both append-only Event triggers. The first API recreation
  correctly failed closed because this worktree's legacy secret mount was empty;
  copying the preserved ignored secret files from the existing deployment source
  restored the exact mount and the same reviewed image became healthy.
- Full DB-backed `go test ./... -count=1 -parallel=1 -p=1`, `go vet ./...`,
  Linux build-tag compilation, Viewer 100 tests/build, 15/15 rollout safety
  tests, SHA-1/SHA-256 Git tests, and the actual-image reaper smoke passed.
- Isolated rollback drilled schema 29 down to 28, served the preserved exact
  `e64c2fbe...` API successfully on schema 28, then migrated back to 29 and
  restored both Event triggers. No live database downgrade was performed.
- Local API health/ready/version, Viewer root/proxy, and tailnet root/API checks
  returned HTTP 200 at schema 29 and exact commit `5a814f3...`. Ports 8080,
  5174, and 8090 remain loopback-only; Tailscale Serve status is unchanged.
  Compact MCP is catalog 1.4.0 with 15 tools and 50 commands; full mode has 89
  tools. UTF-8 round-trip returned `Task Journal 운영 통합·과거 Event backfill`
  exactly with no replacement character.

Tasks #184, #185, and #186 remain `implemented` and unconfirmed. Task #187 is
reported implemented only after this Record and its Run are durably closed; no
Task confirmation was performed. The only residual is the pre-existing Viewer
chunk-size advisory.
