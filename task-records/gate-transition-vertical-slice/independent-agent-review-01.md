---
baley_record: 1
record_id: "19d370c7-03fd-4a6e-8ab6-f0772dead047"
task_id: 110
task_key: "gate-transition-vertical-slice"
record_type: independent-agent-review
run_id: null
created_at: "2026-07-18T01:19:00+09:00"
created_by: "independent-review-agent"
registration_state: registered
supersedes: null
---

# Gate Transition Vertical Slice 독립 Agent 리뷰

## 검토 범위

- 상세계획, System Spec 관련 절과 `contracts/v1/*.json`
- 전체 raw diff와 migration SQL
- PostgreSQL integration, HTTP/MCP와 Viewer 구현
- 승인 재사용, canonical hash, revision/idempotency race, transaction 원자성, cross-Workspace FK, Gate snapshot drift, fixture fallback과 MCP 우회

## 발견사항

### High

1. 동일 idempotency key에 다른 payload를 넣고 과거 승인 hash를 재사용하면 기존 success가 반환될 수 있다. 저장된 요청 fingerprint와 현재 typed request를 비교해야 한다.
2. `gate.pass_task`와 `gate.revoke_task_pass`가 현재 active Phase의 outgoing Gate인지 검사하지 않아 미래 Gate 조건을 변경할 수 있다.
3. Gate의 Phase별 outgoing/incoming 최대 1개 및 인접 Phase 연결 불변식이 DB와 domain 양쪽에 없다.

### Medium

1. idempotency 조회가 Workspace lock보다 앞서 동시 동일-key 실행이 serialization/unique error로 끝날 수 있다.
2. Task/Gate/decision query가 `decisionRequired`, expected revision과 decision snapshot hash를 충분히 반환하지 않는다.
3. `gate.passed` Event 조건 근거에 당시 Task status가 없다.
4. MCP가 HTTP의 `workspace.get`을 노출하지 않아 계획의 query surface와 1:1이 아니다.

## 양호한 항목

- canonical command hash는 contract version, command name, typed arguments, Workspace revision과 Gate snapshot을 포함하며 unknown argument field를 거부한다.
- composite FK는 dependency, Gate와 Gate Task의 cross-Workspace 연결을 차단한다.
- mutation, revision, command, attestation과 Event는 한 PostgreSQL transaction에 있다.
- Viewer는 server 실패를 fixture로 대체하지 않는다.
- MCP는 domain rule을 복제하지 않고 HTTP adapter로 동작한다.

## 보안 한계

V1 HumanApprovalAttestation은 인증된 사람 신원이 아니라 protocol audit metadata다. 로컬 API/MCP 접근을 배포 보안 승인으로 간주해서는 안 된다.
