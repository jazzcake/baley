---
baley_record: 1
record_id: "5890513b-64ad-4e73-914a-3064ebc770b4"
task_id: null
task_key: "gate-transition-vertical-slice"
record_type: review-response
run_id: null
created_at: "2026-07-20T00:11:06+09:00"
created_by: "codex"
registration_state: pending-bootstrap
supersedes: "172fc783-f750-43e5-bb4a-949918aa0aef"
---

# Gate Transition Vertical Slice 리뷰 반영 02

## 결론

독립 Agent 리뷰의 High/Medium finding은 없었다. 비차단 Low 2건은 close-out 전에 모두 보강했다.

## 반영 내용

1. `server/integration/mcp_test.go`가 실제 MCP stdio session에서 `baley_task_report_implemented`로 warning-bearing #110 fixture를 만든 뒤 `baley_task_confirm_preview`와 `baley_task_confirm_execute`를 호출하도록 확장했다. 새 schema 필드의 SDK decode, HTTP forwarding, exact `dangling_path` acknowledgement, 사람 승인 hash 결속과 최종 `confirmed` projection을 한 E2E 흐름에서 검증한다.
2. `server/integration/task_confirm_warning_test.go`의 실패 원자성 assertion에 Task #110의 `implemented` 상태와 `human_approval_attestations = 0`을 추가했다. 기존 Workspace revision, command와 Event 불변 검증도 유지한다.

## 재검증

- 실제 PostgreSQL `TestTaskConfirmWarningAcknowledgementIsAtomicAndRetryable`: 통과.
- 분리된 `baley_test` DB와 loopback API를 사용한 MCP stdio E2E: 통과. `task.confirm` execute가 revision 6과 `implemented → confirmed` projection을 반환했다.
- focused Go test와 `git diff --check`: 통과.
- 테스트용 API process는 종료했으며 live Baley DB는 변경하지 않았다.

## 잔여 위험

- Windows `CGO_ENABLED=0` 환경이라 race detector는 실행하지 못했다.
- V1 HumanApprovalAttestation은 인증된 사람 신원 증명이 아니라 로컬 단일 사용자 protocol audit metadata다.
- production bundle의 기존 500 kB 초과 chunk warning은 남아 있다.
- 현재 Codex thread에 로드된 MCP tool schema는 수정 전 manifest이므로 실제 #110 close-out은 새 thread에서 schema reload 후 수행해야 한다.
