---
baley_record: 1
record_id: "8f80c3ae-40cf-4e91-839f-9a4d417cd1e2"
task_id: 183
record_type: independent-review
run_id: "a31dff3f-3e06-4080-8e8d-120d8928a7db"
created_at: "2026-09-08T18:24:09+09:00"
created_by: "codex-independent-reviewer"
supersedes: "764bd762-d858-4b4a-a07f-ea790d4d551c"
---

# Task #183 독립 재리뷰

## 판정

`CHANGES_REQUIRED`. 첫 리뷰의 blocking 1건과 material 2건은 모두 해소됐다. 다만 full-profile typed backlog promotion 경로에서 새 material 1건이 확인되어, 미해결 blocking 0건·material 1건이다.

## 검토 범위

기준 커밋 `ca44415cd2776981b558755c60a11886a25296ec`, 첫 구현·보고 범위 `6da2c5c1560cb82de5e12480ce1962f393bd1c51..2bfd1d2f8df1307e1affc4d32d64d7597dd8acdb`, 리뷰 수정 범위 `2bfd1d2f8df1307e1affc4d32d64d7597dd8acdb..58937092919c43e2d2e08ac82fdd3e58db41cebe`를 독립 검토했다. 첫 독립 리뷰 원문과 Task #182 산출물은 변경되지 않았고 worktree는 clean했다.

## 해소 확인

- Event source/rebuild: 정규화된 Task Journal seed를 lifecycle Event payload에 먼저 저장하고 persisted Event에서 Journal을 결정적으로 재투영한다. 격리 DB에서 seed Event 7건, Journal 7건, replay mismatch 0건을 확인했다.
- `run.start` cross-key recovery: 원 Task와 정규화 context를 비교해 동일 입력은 기존 command/Event/Journal/lease를 재사용하고 changed/omitted context 및 다른 Task는 `idempotency_conflict`를 반환한다.
- 기존 full-profile typed lifecycle adapter: 첫 리뷰에서 지적한 create/update/run start/implemented/confirm/discard 및 rework/block/unblock 경로에 optional `contextNote` schema·forwarding·부재 호환·approval mismatch 검증이 추가됐다.

## Material finding — typed backlog promotion이 `contextNote`를 유실

`contracts/v1/commands.json`과 `baley-manage-work` skill은 `backlog.promote`를 `task.created` Journal lifecycle로 명시한다. 그러나 `server/cmd/baley-mcp/main.go`의 `backlogPromoteFields`와 preview/execute handler는 optional `contextNote`를 입력받아 `backlogMutationFields`로 전달하지 않는다. catalog schema 및 forwarding tests도 `baley_backlog_promote_preview`와 `baley_backlog_promote_execute`를 대상에서 제외하고 있어 이 누락을 검출하지 못한다.

필수 교정은 두 typed tool 입력 schema와 argument builder/handler에 optional `contextNote`를 추가하고, present forwarding 및 absent-key 호환을 두 경로 모두 검증하는 것이다. 변경된 serialized catalog byte metric, 계약과 문서도 다시 고정해야 한다.

## 독립 검증

- disposable PostgreSQL 17.5에서 migration 26 up/down/up, append-only, lifecycle atomicity/rollback, Event rebuild, same-clientRunId recovery, browser grant 결속을 검증했다.
- cross-key 재시도 후 Run, command, `run.started` Event, `run_started` Journal이 각각 1건임을 확인했다.
- `go test ./... -count=1` PASS, `go vet ./...` PASS.
- UI 17 files/108 tests PASS, production build PASS. 기존 약 1.95 MB chunk warning만 남았다.
- catalog `1.1.0`: compact 15 tools / 4,700 bytes, full 89 tools / 46,217 bytes.
- 검토용 PostgreSQL container와 로컬 검토 binary는 제거했다. 운영 DB·서비스·방화벽·Tailscale·branch·push/merge/deploy는 변경하지 않았다.

## 재리뷰 계약

위 material 1건을 수정하고 전체 회귀 검증을 다시 통과한 뒤 동일 독립 리뷰어가 재검토한다. unresolved blocking/material finding이 모두 0이 되기 전에는 Task #183을 `implemented`로 전환하지 않는다.
