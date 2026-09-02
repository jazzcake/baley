---
baley_record: 1
record_id: "396e9e96-e944-4991-97f5-bc3f7c104ef5"
task_id: 162
task_key: "mcp-login-membership-auth"
record_type: review-response
run_id: "300eadf0-6c78-4d64-a99c-2e0657cdffbc"
created_at: "2026-09-02T15:05:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: "089ab381-b0a0-462e-875a-cb471ea2daaf"
---

# #162 v6 stdio reboot remediation

## Outcome

The retired per-session stdio registration has been removed from both the
standalone Codex home and the Orca private runtime home. A reboot now starts one
scheduled loopback Gateway, and restored Codex/Orca sessions resolve `baley` to
`http://127.0.0.1:8090/mcp` without an environment token or header.

## Changes

- The Windows installer registers and verifies every applicable Codex home:
  the process `CODEX_HOME`, the default `%USERPROFILE%\.codex`, the discovered
  Orca runtime home, and optional explicitly supplied homes.
- Candidate paths are absolute and case-insensitive duplicates are removed.
- Every target is verified with `codex mcp get baley` for both the exact URL and
  `streamable_http` transport.
- The caller's process `CODEX_HOME` is restored on success and failure.
- A reusable reconnect prompt tells existing sessions to preserve work, mark
  only genuinely completed Runs succeeded, interrupt incomplete Runs, avoid the
  retired binary/config, and fresh-read their work after reboot.

## Live verification

- The installer was run with process `CODEX_HOME` deliberately removed.
- It discovered and verified both `C:\Users\jazzc\.codex` and
  `C:\Users\jazzc\AppData\Roaming\orca\codex-runtime-home\home`.
- Both homes report URL-only Streamable HTTP configuration with no bearer token,
  HTTP header, environment header, credential-store variable, or old executable.
- The original process environment was restored.
- The AtLogOn scheduled task runs one release binary from
  `C:\dev-bin\baley\releases\8012842eb00c\baley-mcp.exe serve-http` on
  `127.0.0.1:8090`.
- Existing stdio children are deliberately left alive for the user to close;
  no persisted configuration can recreate them after reboot.
- Independent focused re-review passed with no finding in this remediation.

## Residual finding

The separate Medium finding in `independent-agent-review-02.md` remains open:
a completed pending login link must be invalidated or session-revalidated when
logout happens before callback redeem. Task #162 must not be confirmed until
that security defect is fixed and reviewed.
