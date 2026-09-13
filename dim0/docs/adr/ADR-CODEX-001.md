# ADR-CODEX-001: Codex as an additive AI runtime

## Status

Accepted for the Baley-hosted Dim0 fork.

## Decision

Keep Dim0 independent from Baley and preserve the browser-agent HTTP contract. `DIM0_AI_RUNTIME` selects the runtime; upstream provider behavior remains the code default, while `build/docker-compose.codex.yml` explicitly selects `codex`.

The backend owns one persistent `codex app-server` child over newline-delimited stdio and maps each `X-Run-Id` to a Codex thread. Structured output chooses either content or a browser-executed canvas tool call; Codex does not execute canvas tools itself.

Each turn uses `approvalPolicy=never`, a read-only sandbox restricted to an empty directory, and no project mount. `CODEX_HOME` is the only runtime-state mount and must be outside Git. Requests use the account authenticated there and may incur account usage.

## Scope boundary

No Baley task reference, synchronization, schema/API change, Bird's View migration, documentation integration, or UI embedding is introduced. The legacy server-side assistant remains upstream code.

## Upstream maintenance

Imported from `vcmf/dim0@c75cb32901aac30b1dd79f07d4a560d93b21750b` as a squash subtree under `dim0/`. Refresh from the Baley root with:

```powershell
git subtree pull --prefix=dim0 <local-dim0-clone> main --squash
```

Keep fork resolutions within `ai_runtime/`, the two `/ai/llm` branches, and the additive Docker overlay.
