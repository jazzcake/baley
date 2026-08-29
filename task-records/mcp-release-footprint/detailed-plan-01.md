# Detailed plan — MCP release footprint

- Record ID: `b1b17a53-8dab-4937-ae91-d08375b7fb0d`
- Task: #152 MCP 릴리스 바이너리 경량화 및 메모리 프로파일링
- Run: `f7c9bad1-e278-4558-a352-73f7cea92325`

## Objective

Make the installed `baley-mcp` release artifact smaller and make its resource
footprint observable without weakening the tokenless stdio boundary. A stdio
MCP client intentionally owns one local process per Codex session; this task
does not add a shared listener or an externally reachable transport.

## Implementation plan

1. Build installed Windows and macOS artifacts with `-trimpath` and stripped
   Go symbol/debug tables (`-ldflags "-s -w"`).
2. Add a read-only Windows diagnostic that reports the installed artifact size
   and the working-set/private memory of running `baley-mcp` sessions.
3. Document the distinction between on-disk size and per-process memory, along
   with the diagnostic entry point.
4. Compare controlled stripped and unstripped builds and smoke-test the
   stripped binary's MCP diagnostic command.
5. Run focused Go tests plus the repository validation suite; record the
   results, obtain an independent review, and publish a completion report.

## Safety boundaries

- Do not restore `BALEY_MCP_GATEWAY_TOKEN` or write any Authorization header
  into Codex configuration.
- Do not change Account, membership, Task/Gate human-only approval, or audit
  semantics.
- Keep all local transport tokenless stdio and loopback-safe.
