---
baley_record: 1
record_id: "773667ba-754b-4cc0-b830-820f8ba66d1a"
task_id: 131
task_key: "gate-public-number-alias"
record_type: detailed-plan
run_id: "fc635578-f935-4ba2-81cf-219fa97f2bde"
created_at: "2026-07-29T23:18:41+09:00"
created_by: "codex"
registration_state: registered
supersedes: null
---

# Task #131 상세 구현 계획

## 목표

Gate의 내부 `gateId`는 안정 식별자로 유지하면서 Workspace별 재사용되지 않는
`publicId`와 선택적 `alias`를 추가한다. 운영자는 `G#<publicId>`, `gateId`, `alias`
중 하나로 Gate를 조회·지시할 수 있고 Viewer는 `G#<publicId>`를 우선 표시한다.

## 구현

1. Migration 15에서 `gates.public_id`, `gates.alias`,
   `workspace_counters.next_gate_public_id`를 추가한다.
2. 기존 Gate 번호는 Workspace별 from-Phase 순서와 gateId 순서로 결정적으로 backfill한다.
3. public ID와 대소문자를 무시한 alias를 Workspace 범위에서 유일하게 강제한다.
4. Gate 생성 시 counter를 CAS로 증가시키고 선택적 alias를 저장한다.
5. 공용 Gate reference resolver가 `G#번호`, 내부 gateId, alias를 해석하게 한다.
6. HTTP, CLI, MCP의 기존 `gateId` 입력 호환성은 유지하고 alias 입력과 출력을 확장한다.
7. Viewer Gate 카드·Inspector·Task 관계에 `G#번호`, alias, 내부 gateId를 표시한다.

## 검증

- Migration up/down, deterministic backfill, counter/unique constraint 통합 테스트
- Gate 생성·조회·pass/attach reference 회귀 테스트
- CLI와 typed MCP schema/forwarding 테스트
- 프런트엔드 API 변환·Gate 표시 테스트
- 전체 Go test, `go vet`, 프런트엔드 테스트, production build
- 독립 Agent 리뷰 후 발견 사항 반영
