---
baley_record: 1
record_id: "cc395fe1-5fd4-4135-92df-02bc0a9bbf66"
task_id: null
task_key: "run-record-vertical-slice"
record_type: independent-agent-review
run_id: null
created_at: "2026-07-18T11:14:04+09:00"
created_by: "independent-review-agent"
registration_state: pending-bootstrap
supersedes: null
---

# Run/Record Vertical Slice Wave 1 독립 Agent 리뷰

## 범위

- System Spec의 Run, transaction과 필수 인수 테스트
- `contracts/v1`의 states, diagnostics와 commands
- Phase 2–4 선행 구현 계획의 Wave 1
- Wave 1 raw diff와 Run 관련 domain 코드·테스트
- Wave 1 완료보고

리뷰 Agent는 파일을 수정하지 않았고 다음 검증을 실행했다.

```text
go test -count=1 ./internal/domain
go vet ./internal/domain
```

두 명령은 통과했다.

## Findings

### Medium 1 — heartbeat가 lease와 시간을 단조 증가시키지 않는다

`Run.Heartbeat`는 새 `now`가 기존 `HeartbeatAt`보다 이전인지 확인하지 않고 `LeaseExpiresAt`을 `now + extension`으로 덮어쓴다. 기존 만료보다 짧은 extension은 heartbeat가 lease를 연장하는 대신 단축할 수 있다.

필요 조치:

- `now >= HeartbeatAt` 강제
- 새 lease expiry가 기존 expiry보다 뒤인지 강제하거나 server-owned extension 정책으로 고정
- 시간 역행과 lease 단축 회귀 테스트

### Medium 2 — closed Workspace에서 Run start plan을 차단하지 않는다

현재 planner는 Workspace lifecycle 상태를 입력받지 않으며 completed Phase의 모든 Run kind를 허용한다. 마지막 Phase가 completed된 closed Workspace의 비terminal Task에도 plan이 만들어질 수 있다.

필요 조치:

- Workspace 상태를 planner 입력에 포함하거나 application 계층의 필수 선행 검사로 명시
- closed Workspace hard error 테스트

### Medium 3 — 독립 리뷰 target Run의 구조적 관계를 검증하지 않는다

`target_run_id`를 그대로 복사하므로 자기 자신, 다른 Workspace/Task의 Run 또는 implementation이 아닌 Run도 target이 될 수 있다.

필요 조치:

- 알려진 target Run context를 planner에 제공
- target 존재, Workspace/Task, implementation kind와 self-reference 검증
- target 미보고는 계속 허용

### Low 1 — Run 상태 계약 검사가 양방향이 아니다

현재 contract test는 domain 값이 JSON 계약에 존재하는지만 확인한다. JSON에 새 status/kind가 추가되고 Go 모델이 빠져도 통과한다.

필요 조치:

- Run status와 kind 집합의 양방향 완전 일치 검사

### Low 2 — heartbeat와 terminal의 순수 CAS 경쟁 시퀀스가 직접 시험되지 않는다

각 mutation의 stale version 테스트는 있지만 heartbeat 적용 후 예전 version의 terminal 거부, terminal 적용 후 heartbeat 거부가 하나의 경쟁 시나리오로 고정되지 않았다.

필요 조치:

- 두 실행 순서의 명시적 회귀 테스트

## 결론

High finding은 없다. 자체 단위 테스트는 통과하지만 Medium 3건이 열려 있으므로 Wave 1을 독립 리뷰 완료로 close하지 않는다. Wave 2 구현과 병렬로 전진할 수는 있으나, Wave 1 persistence adapter 또는 통합 검증 전에 findings를 반영해야 한다.
