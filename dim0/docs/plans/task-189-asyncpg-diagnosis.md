# Task #189 Stage D asyncpg diagnosis

Date: 2026-09-14 KST

Diagnosis target: commit `06a9c84` and external evidence
`C:\ProgramData\Dim0\validation\task-189\run-20260914-053311-422-bc2bc812`

Scope: diagnosis only; no application or harness source was changed.

## Conclusion

The first observable divergence is inside asyncpg's native simple-query protocol,
not in FastAPI, a row codec, or PostgreSQL schema validation. `apply_schema()`
submitted the complete `schema.sql` string through `Connection.execute()`. When
PostgreSQL's `ReadyForQuery` was dispatched, asyncpg 0.30.0 entered
`_on_result__simple_query()` with `result_status_msg is None` and attempted
`None.decode("utf-8")` at `protocol.pyx:815`. That field is initialized/reset to
`None` and is populated only by a parsed `CommandComplete` message. The failing
client therefore reached result dispatch without a retained command-completion
status.

The evidence is sufficient to locate that divergence with high confidence, but
not to prove why the status was absent. The same retained database, unchanged
schema, DSN shape, CPython 3.13.15, and asyncpg 0.30.0 now pass the isolated
probe. The recorded failure is thus not a persistent schema/data/configuration
failure and was not reproduced after the Task #189 PostgreSQL process was
recreated. A transient Windows asyncpg/proactor/C-extension protocol-state
failure remains plausible, but the captured run contains neither wire-level
messages nor asyncpg protocol-state instrumentation needed to distinguish it
from a transient transport/cancellation event.

There is also a definite harness boundary error: Stage D's ledger command was
host-native `uv run pytest`, so the failure came from the Windows CPython wheel,
while Stage B had built a Linux backend image and the intended application stack
uses that image. Describing this result as a container baseline failure conflates
two different runtimes.

## Captured evidence

- The 344-line `40-storage-contract.txt` (SHA-256
  `2e3ac8891d89643e1c954a2341fabf4936ec76a3920e439f64dbc01a25dc3f5a`)
  contains two independent test failures at the same path:
  `FastAPI lifespan -> apply_schema -> conn.execute -> Protocol.query ->
  BaseProtocol._dispatch_result -> protocol.pyx:815`.
- Both traces are Windows paths under
  `cpython-3.13-windows-x86_64-none` and `.venv\Lib\site-packages`; neither
  trace came from `/app` in the Linux backend container.
- PostgreSQL was healthy before and after the failures. The bounded server log
  contains normal startup/readiness only, with no `ERROR`, `FATAL`, or rejected
  statement corresponding to either failure.
- The command ledger records the exact Stage D command as
  `uv run pytest -q test/integration/baseline/test_provider_free_baseline.py`.
  It does not use `docker compose run`, `exec`, or the built backend image.
- The three Task #189 persistence volumes were retained after the run. No other
  Docker project was inspected or changed during this diagnosis.

## Runtime and connection inventory

| Layer | Observed value |
| --- | --- |
| Failing Stage D host | Windows 11, CPython 3.13.15, asyncpg 0.30.0, FastAPI 0.128.0, pytest 8.4.1, pytest-asyncio 1.1.0 |
| Host asyncpg artifact | Official wheel tag `cp313-cp313-win_amd64`; protocol extension SHA-256 `a866edd7ec2d40ee0a035983632776706031f208aa2925df6da81d9c9870b169` |
| Built backend image | `dim0-task189-backend-test:latest` (`86945499d37d`), Linux/amd64, CPython 3.13.15, asyncpg 0.30.0, FastAPI 0.128.0 |
| Task database | `postgres:15` image `9b1d34adbce1`, PostgreSQL 15.19, Debian x86_64 |
| Encoding | server `UTF8`, client `UTF8`, `lc_messages=en_US.utf8` |
| Authentication | Task-only loopback port, role/database `topix`, Compose `POSTGRES_HOST_AUTH_METHOD=trust`; no password |
| Sanitized host DSN | `postgresql://topix@localhost:15434/topix` |
| Container DSN shape | `postgresql://topix@postgres-test:5432/topix` |
| Pool parameters | `min_size=5`, `max_size=25`, acquire timeout 10 s, command timeout 30 s |
| Custom codecs/settings | None. The repository has no `set_type_codec`, `set_builtin_type_codec`, asyncpg pool `init`, SSL override, or server-settings override. |

The external `baseline.env` contains the expected Task #189 loopback ports and
provider controls. All provider credential values are empty; no secret value was
printed or copied into this report. `PostgresConfig.dsn()` URL-encodes user and
optional password and otherwise supplies only host, port, database, and user.

## Schema and retained data

`apply_schema()` reads the 8,149-byte UTF-8 `build/schema.sql` and sends it as
one multi-statement simple query inside an asyncpg transaction. The file contains
nine `CREATE TABLE IF NOT EXISTS` statements, indexes, one idempotent root-user
insert, and five additive `ALTER TABLE` statements. The last command is
`ALTER TABLE user_billing ADD CONSTRAINT ...`; successful execution therefore
returns the final status `ALTER TABLE`.

Read-only inspection of `dim0-task189_pg_data_test` found PostgreSQL's standard
`plpgsql` extension, the expected twelve application tables (including the two
store-created tables), and the expected 43 public constraints. The retained
volume contained one root user and one graph, with no board-oplog rows. That
pre-existing graph is not referenced by schema bootstrap, and the full schema
completed against it in every diagnostic probe. Retained state therefore does
not explain the protocol exception.

## Minimal probes

Only `dim0-task189_pg_data_test` was started, through the original Task #189
Compose project. Qdrant, Redis, backend, and Web UI were not started. Each schema
probe ran within an explicit transaction that was rolled back.

1. On the Windows host, asyncpg 0.30.0 successfully executed `SELECT 1`, a
   two-statement query, a query with an empty trailing statement, and a query
   with a trailing comment.
2. The Windows host successfully executed the full unchanged `schema.sql` and
   returned `ALTER TABLE`.
3. A closer reproduction of the test boundary (connect/close readiness probe,
   create a 5-connection pool, acquire, execute full schema, rollback, close)
   succeeded 20 of 20 times with `ALTER TABLE`.
4. After installing the provider tripwire imports in-process, the same Windows
   readiness/pool/schema rollback probe also returned `ALTER TABLE`.
5. The locked Linux backend image, on the Task #189 Compose network, ran the
   same full-schema rollback against the same retained PostgreSQL volume and
   returned `ALTER TABLE` with CPython 3.13.15 and asyncpg 0.30.0.

The diagnostic container and network were removed afterward without `-v`; all
three originally retained Task #189 volumes remain.

## Hypothesis assessment

| Hypothesis | Assessment | Evidence |
| --- | --- | --- |
| Malformed DSN/environment | Ruled out (high confidence) | Readiness connections succeeded in the captured test; sanitized DSN is valid; direct and pooled probes use the same endpoint and pass. |
| Schema syntax or unsupported PostgreSQL statement | Ruled out (high confidence) | PostgreSQL reports no SQL error, and the entire unchanged file repeatedly returns `ALTER TABLE` on the retained DB. |
| Custom codec/decoder bug in application setup | Ruled out (high confidence) | Failure is status-message decoding in simple-query dispatch before stores open; no application asyncpg codecs exist. |
| Stale retained schema/data | Ruled out (high confidence) | Expected objects/constraints exist and rollback probes pass against the same retained volume; both old and current data shapes are accepted. |
| General Python 3.13 incompatibility | Ruled out (moderate-high confidence) | The identical CPython 3.13.15/asyncpg 0.30.0 pair passes on both Windows and Linux now. This does not rule out a Windows-only intermittent compatibility defect. |
| Broken or wrong asyncpg build | No persistent defect found (moderate confidence) | The host loads the correct `cp313-win_amd64` wheel, unchanged since before the baseline, and passes 20/20 pool probes. A transient native-protocol defect remains possible. |
| PostgreSQL 15 incompatibility | Ruled out (high confidence) | Both platform clients successfully execute the schema against PostgreSQL 15.19. |
| Provider-tripwire mutation | Ruled out (high confidence) | The tripwire does not patch asyncpg; an import/installation-equivalent probe passes. |
| Harness invocation/platform mismatch | Confirmed (high confidence) | Stage D ran the Windows host environment despite building a locked Linux backend image; the captured trace proves the mismatch. |

## Root-cause confidence

- **First divergence: high (0.95).** asyncpg simple-query dispatch attempted to
  decode an unset `result_status_msg`; this is directly established by the trace
  and the installed 0.30.0 source at lines 815 and 842-888.
- **Underlying transient mechanism: low-moderate (0.40).** The original bundle
  lacks protocol/wire/cancellation instrumentation, and the retained-resource
  reproduction no longer fails.
- **Harness causality for the invalid baseline claim: high (0.95).** Stage D
  tested a host-native Windows C extension rather than the locked Linux artifact
  whose behavior the Compose baseline was intended to establish.

## Minimal recommended fix

Change only the baseline harness in a follow-up task so Stage D executes the
focused pytest contract inside the already-built `dim0-task189-backend-test`
image on the Task #189 Compose network, with the same explicit non-secret
environment and Task-only persistence services. Record Python, asyncpg, wheel or
extension identity, PostgreSQL version, sanitized DSN shape, and a bounded
asyncpg protocol/transport diagnostic if the failure recurs. This removes the
confirmed platform mismatch and makes a future failure attributable to the
shipped runtime.

Do **not** change schema statements or pin/downgrade Python based on this bundle.
Do **not** upgrade asyncpg as the first response: compare 0.30.0 with the proposed
version only if the containerized probe reproduces the missing-status condition.
No product fix is justified by the currently reproducible evidence.
