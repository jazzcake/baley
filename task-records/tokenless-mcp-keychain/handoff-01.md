---
baley_record: 1
record_id: "3f6c6051-3c8c-487a-bc05-3703e3113583"
task_id: 149
task_key: "tokenless-mcp-keychain"
record_type: handoff
run_id: "18b2c250-e49c-44ff-bf17-e079d3f7b9a2"
created_at: "2026-08-28T00:00:00Z"
created_by: "codex"
supersedes: null
---

# #149 Codex implementation handoff prompt

Implement and close out Baley Task #149, **OIDC/OAuth 연동 후 Gateway token 제거·Keychain 기기 비밀 전환**, in this worktree.

Start by reading `AGENTS.md`, `.agents/skills/baley-manage-work/SKILL.md`,
and `task-records/tokenless-mcp-keychain/detailed-plan-03.md`. The Task is
already in progress; do not claim it confirmed. Work from the current branch,
preserve unrelated changes, and inspect the existing implementation before
editing. The required outcome is that Codex Desktop and CLI use Baley via
tokenless local stdio and OS-keychain-protected device secrets, with safe
migration/rollback/diagnostics and immediate invalidation on logout, revoke,
membership removal, or gateway replacement.

Required proof: test a fresh process without `BALEY_MCP_GATEWAY_TOKEN`, verify
the actual Codex MCP registration has no Authorization header or token env,
run focused tests plus full Go/frontend/build verification, deploy, and smoke
test the deployed route. Do not expose any secret. Do not weaken Task/Gate
human-only authorities. Add a completion report Task Record, run an
independent-agent review and response, commit and push. Send a concise
completion report with commit SHA, deployed verification, test results, record
paths, remaining risk, and whether #149 can truthfully be reported
implemented (never confirmed).

