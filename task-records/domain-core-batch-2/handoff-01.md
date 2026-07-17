---
baley_record: 1
record_id: "1df95631-4262-4a86-9874-8de6a1fbe30c"
task_id: null
task_key: "domain-core-batch-2"
record_type: handoff
run_id: null
created_at: "2026-07-17T21:53:20.5632496+09:00"
created_by: "codex"
registration_state: pending-bootstrap
supersedes: null
---

# Domain Core Batch 2 구현 Handoff

당신은 Baley의 두 번째 Go domain core 배치를 구현하는 세션이다. 목표는 기능 수를 늘리는 것이 아니라 Task lifecycle, blocker와 dependency patch의 V1 불변식을 순수 Go 코드와 테스트로 고정하는 것이다.

## 반드시 먼저 읽을 파일

다음 경로만 정확히 읽고 시작한다. `task-records/` 전체를 검색하지 않는다.

1. `task-records/domain-core-batch-2/detailed-plan-01.md`
2. `docs/baley-system-spec-v1.md`
   - §8 Task
   - §9 Task 구조와 dependency
   - §15 Command와 MCP
   - §16 Transaction과 Event
   - §20 구현 순서
   - §21 필수 인수 테스트
3. `contracts/v1/states.json`
4. `contracts/v1/diagnostics.json`
5. `contracts/v1/commands.json`
6. `docs/baley-command-architecture.md`
7. `server/internal/domain/`의 현재 Go 코드와 테스트

정본 우선순위는 System Spec의 도메인 의미, `contracts/v1`의 literal, Command Architecture의 파생 설명 순이다.

## 작업 전 안전 규칙

- 현재 branch와 `git status --short`를 확인한다.
- 사용자가 명시하지 않았으므로 branch나 worktree를 만들지 않는다.
- 기존 dirty working tree는 사용자 작업으로 취급하고 보존한다.
- 이번 범위와 무관한 UI·문서 변경을 되돌리거나 정리하지 않는다.
- 커밋과 push는 사용자가 명시적으로 지시하기 전까지 하지 않는다.
- 파일 편집은 가능한 한 `apply_patch`를 사용한다.

## 구현 범위

다음을 구현한다.

1. 공통 Diagnostic/Evaluation 모델
2. Task typed status와 lifecycle transition
3. blocker metadata와 실행 가능성 정책
4. WorkspaceGraph aggregate
5. atomic dependency patch
6. root/leaf/dangling path preview diff
7. 기존 connect 동작을 patch convenience operation으로 수렴
8. 계약 literal 일치 테스트

PostgreSQL, HTTP API, MCP, Event persistence, HumanApprovalAttestation, Gate pass, Run lease와 Viewer는 구현하지 않는다.

## 고정 구현 결정

- dependency는 같은 Workspace 안에서 Lane과 Phase를 넘을 수 있다.
- 뒤 Phase에서 앞 Phase로 향하는 dependency는 성공하면서 `phase_order_inversion` warning을 반환한다.
- cycle, self-link, duplicate와 cross-Workspace 연결은 hard error다.
- dependency는 명시적 Gate–Task 연결 없이는 Gate readiness에 영향을 주지 않는다.
- Task status와 blocker는 별도다.
- block은 pending/in_progress에만 허용한다.
- implemented Task는 rework 후 block한다.
- blocker는 기존 Run을 자동 중단하지 않는다.
- blocker는 새 implementation/review-response와 구현완료 보고를 막는다.
- 상세계획, 독립 Agent 리뷰와 완료보고는 blocker/dependency 때문에 막지 않는다.
- warning acknowledgement는 이후 command service의 책임이다.
- `dangling_path`는 patch 실패 조건이 아니다.
- 실패한 patch와 transition은 입력 상태를 변경하지 않는다.

명세와 위 결정이 충돌한다고 판단되면 임의로 확장하지 말고 충돌 위치와 제안만 보고한다.

## 권장 구현 순서

1. Diagnostic 타입과 contract test를 먼저 정리한다.
2. Task 상태 머신과 blocker 테스트를 작성하고 구현한다.
3. 현재 DependencyGraph를 WorkspaceGraph로 확장한다.
4. patch input, candidate copy, 최종 graph validation을 구현한다.
5. connect/disconnect/terminal convenience operation을 patch에 연결한다.
6. preview diff와 path projection을 추가한다.
7. Run 시작 가능성 순수 정책을 추가한다.
8. 전체 Go 테스트를 정리하고 프론트 회귀 검증을 수행한다.

## 테스트에서 반드시 증명할 것

- 정상 Task transition과 전체 invalid transition
- terminal 상태 불변
- blocker 상태 범위와 사유 검증
- blocker/dependency별 Run kind 허용 차이
- 복수 edge patch와 방향 전환
- candidate cycle 발견 시 전체 rollback
- patch 실패 후 기존 edge 보존
- cross-lane/cross-Phase 허용
- 역방향 Phase warning
- terminal reason과 outgoing edge/Gate 조건 배타성
- root/leaf/dangling 변화
- Go literal과 JSON 계약 일치

## 필수 검증 명령

```powershell
Set-Location server
$files = Get-ChildItem -Path internal/domain -Filter *.go | ForEach-Object { $_.FullName }
gofmt -w $files
go test ./...
go test -race ./...
go vet ./...

Set-Location ..
npm test -- --reporter=dot
npm run build
```

Windows에서 `gofmt -w internal/domain/*.go`는 wildcard가 그대로 전달될 수 있으므로 위처럼 파일 목록을 넘긴다.

## 독립 리뷰와 반영

구현과 1차 검증이 끝나면 가능한 경우 독립 Agent에게 다음 원자료만 제공해 리뷰를 요청한다.

- `task-records/domain-core-batch-2/detailed-plan-01.md`
- System Spec의 관련 절
- `contracts/v1/*.json`
- 구현 raw diff
- 테스트 출력

리뷰 Agent에게 예상 결론이나 의심하는 버그를 미리 알려주지 않는다. 리뷰 결과는 새 UUID를 가진 다음 파일로 기록한다.

```text
task-records/domain-core-batch-2/independent-agent-review-01.md
```

리뷰 반영 내용은 별도 `review-response-01.md`에 기록한다. 발견을 단순히 숨기기 위해 명세를 완화하지 않는다. 명세 변경이 타당하면 이유와 영향을 먼저 보고한다.

## 완료보고

리뷰 반영과 전체 재검증 뒤 다음 파일을 작성한다.

```text
task-records/domain-core-batch-2/completion-report-01.md
```

완료보고에는 최소 다음을 포함한다.

- 구현한 범위
- 변경 파일
- 테스트와 build 결과
- 독립 리뷰 발견과 반영 결과
- 남은 위험과 기술부채
- 명세와 구현이 아직 어긋나는 부분
- 다음 배치 제안

Baley가 구현 품질을 인증했다고 표현하지 않는다. 구현 주체의 판단과 검증 결과를 구분해 기록한다.

## 세션 완료 응답

사용자에게는 결과부터 간결하게 보고한다.

- 구현 결과
- 검증 결과
- 리뷰 결과
- 남은 위험
- 작성한 Task Record의 정확한 경로
- 커밋하지 않았다는 사실
