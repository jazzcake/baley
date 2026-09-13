# Spec: Codex runtime with upstream Dim0 persistence

Status: **draft for operator review — normative basis for implementation**.

This document supersedes `codex-only-runtime-spec.md`.

## 1. Product and repository boundary

The Baley-hosted Dim0 fork is an independent spatial thinking workspace. It lives under `dim0/` as a squash Git subtree but runs as a separate web service. This phase introduces no Baley task reference, synchronization, schema/API change, Bird's View migration, documentation integration, or Baley UI embedding.

The fork minimizes divergence from upstream. PostgreSQL, Qdrant, Redis, canvas/node/edge behavior, document processing, and the browser-owned canvas tool loop remain structurally unchanged.

## 2. Target runtime architecture

```text
Browser
  -> Dim0 Web UI
  -> FastAPI backend
       |-- PostgreSQL (metadata, identity, permissions, logs)
       |-- Qdrant (content payloads, vectors, nodes, edges, documents, chunks)
       |-- Redis (tickets, sequence and quota state)
       `-- CodexRuntime -> persistent codex app-server
```

The Codex profile is selected explicitly with `DIM0_AI_RUNTIME=codex`.

## 3. Generative LLM policy

### 3.1 Sole agent-generation path

In Codex mode, every generative agent turn MUST use:

```text
Dim0 /ai/llm[/stream] -> CodexRuntime -> codex app-server
```

OpenAI, Anthropic, OpenRouter, Gemini, Mistral, DeepSeek, or another direct LLM completion MUST NOT be selected or invoked. Codex failure MUST surface as an error; there is no LiteLLM, OpenAI Agents SDK, BYOK LLM, or `codex exec` fallback.

“Codex runtime” is not offline inference. The locally running app-server may communicate with the Codex service through the account authenticated in its dedicated `CODEX_HOME`.

### 3.2 Fail-closed LLM selection

In Codex mode:

- the catalog MUST expose no direct-provider **LLM** route to execution code;
- `resolve_code()`, `default_resolved()`, and `require_model_code()` MUST NOT return an LLM provider call code;
- the UI MUST choose the managed Codex path even if OpenAI/Anthropic BYOK credentials are stored;
- the legacy server-side assistant and automatic provider classifier MUST be disabled unless adapted to `CodexRuntime`;
- remaining `litellm.acompletion()` and `LitellmModel` boundaries MUST fail before network I/O.

Provider keys may remain present for embeddings and non-LLM services, but MUST NOT re-enable direct LLM generation.

## 4. Qdrant and embedding policy

### 4.1 Qdrant remains mandatory

Qdrant is not treated as an optional RAG add-on. The existing `GraphStore -> ContentStore -> Qdrant` path remains the canonical storage path for substantial canvas content, including notes, links, documents, chunks, and graph node/edge payloads.

This phase MUST NOT remove Qdrant, migrate content to PostgreSQL, introduce a replacement store abstraction, tune Qdrant memory, or redesign semantic/RAG architecture.

### 4.2 Current embedding coupling

The current implementation has these load-bearing behaviors:

- `GraphStore()` constructs `ContentStore.from_config()`.
- `ContentStore.from_config()` constructs `OpenAIEmbedder.from_config()` immediately.
- only OpenAI and OpenRouter currently provide the configured `text-embedding-3-small` route at 512 dimensions.
- `ContentStore.add()` embeds before Qdrant upsert.
- `ContentStore.update()` regenerates vectors before updating payload/vector data.
- note creation, link creation, document/chunk ingestion, text changes, and semantic queries can therefore call the embedding API.
- spatial/style-only note patches use `update_payload_only()` when embeddable text is unchanged and do not call the embedding API.
- if no embedding provider is configured, backend store construction fails; if embedding fails during add/update, the corresponding content write does not complete normally.

### 4.3 Embedding decision for this phase

Remote embedding is an explicitly retained external dependency, separate from agent LLM generation. The fork MAY use `OPENAI_API_KEY` or `OPENROUTER_API_KEY` for the existing embedding route while direct LLM completion remains prohibited in Codex mode.

No local embedding replacement, zero-vector fallback, deferred embedding queue, or persistence/embedding decoupling is introduced now. Those are separate architecture tasks if actual operation demonstrates a need.

## 5. Other external AI tools

Dim0's existing search (Perplexity, Tavily, Linkup, Exa), URL fetch, Mistral OCR/document parsing, and Daytona remote-code services are non-LLM agent tools. This change MUST NOT disable, reroute, or otherwise alter their existing resolution, credentials, confirmation gates, or behavior.

An `X-Provider-Key` used for one of these services remains valid under its existing contract. It MUST NOT enable a direct LLM completion path.

## 6. Persistence and Docker requirements

- PostgreSQL and Qdrant persistence remain compatible with upstream Dim0.
- Qdrant runs as a dedicated Dim0 dependency with its own persistent volume.
- Redis remains present because the backend currently depends on it for tickets, sequence allocation, and quotas.
- The Desktop Docker profile builds the fork's backend and Web UI locally.
- Existing PostgreSQL may be reused only through an explicit external-database configuration; Dim0 MUST use a dedicated database and role, and startup schema application must remain idempotent.
- When external PostgreSQL is selected, Compose MUST not require or start its bundled PostgreSQL service. Qdrant and Redis remain independently managed by the Dim0 stack unless explicitly externalized later.
- `CODEX_HOME` MUST be a dedicated persistent directory outside Git and be the only Codex state/credential mount.
- The Codex working directory is an empty dedicated directory with restricted read-only access, no writable project root, and `approvalPolicy=never`.

## 7. Codex session lifecycle

- One long-lived `codex app-server` process per backend worker.
- One initialization handshake per process connection.
- Existing `X-Run-Id` maps to a Codex thread for the duration of a browser-agent run.
- The browser continues to execute canvas tools; Codex returns a structured content response or next tool call.
- Process exit, authentication failure, timeout, malformed output, cancellation, and backend shutdown have explicit handling.
- No failure path falls back to a direct LLM provider.

## 8. Acceptance criteria

1. Upstream baseline runs with Web UI, backend, PostgreSQL, Qdrant, and Redis before the runtime cutover is judged.
2. In Codex mode, provider keys present for embedding do not make direct LLM routes executable.
3. Test doubles record zero LiteLLM, OpenAI Agents, classifier, and browser BYOK LLM invocations in Codex mode.
4. The existing embedding client is constructible and its route is reported separately from LLM availability.
5. Note/link creation and retrieval persist through Qdrant and survive container restart.
6. Text creation/update invokes embedding; spatial/style-only updates do not.
7. Embedding failure behavior is observed and documented without redesigning it.
8. `/ai/llm` and `/ai/llm/stream` use only the injected Codex runtime and never provider fallback.
9. A Codex-selected canvas tool call completes through the existing browser loop and creates/updates the expected node or edge.
10. Search, fetch, OCR, and Daytona resolution behave identically with and without Codex mode.
11. Bundled-PostgreSQL and external-PostgreSQL Compose configurations both validate; the latter uses a dedicated database/role.
12. No credential, `CODEX_HOME`, PostgreSQL data, or Qdrant data is written inside the repository.

## 9. Minimal implementation plan

1. Record an upstream baseline smoke test using the unmodified provider runtime and bundled persistence services.
2. Split catalog LLM availability from embedding availability so Codex mode suppresses only LLM routes.
3. Add a small runtime-policy guard at remaining direct LLM invocation boundaries.
4. Disable the legacy server agent and browser/desktop BYOK LLM selection in the Codex build profile only.
5. Preserve `OpenAIEmbedder`, `ContentStore`, `GraphStore`, Qdrant collection shape, and all non-LLM tool services.
6. Harden the existing Codex adapter process/session/error lifecycle without restructuring upstream agent or canvas domains.
7. Provide two additive Docker overlays: bundled PostgreSQL and external PostgreSQL. Both retain Qdrant and Redis.
8. Add the acceptance tests above, then run Docker and browser end-to-end validation.

## 10. Explicitly deferred work

- Baley integration of any kind;
- Qdrant removal or PostgreSQL-only storage;
- storage abstraction rewrite or memory tuning;
- semantic/RAG redesign;
- embedding subsystem replacement or decoupling;
- Bird's View migration;
- Dim0 embedding inside the Baley UI.

## 11. Upstream merge constraint

Fork changes SHOULD remain additive and concentrated in the runtime-policy/adapter, existing provider-selection seams, tests, and Docker overlays. Upstream provider and persistence implementations remain in place and continue to work outside the explicit Codex profile.
