---
baley_record: 1
record_id: "e1408907-508e-43dc-9b4e-ceabc7b722bf"
task_id: null
task_key: "domain-core-batch-2"
record_type: completion-report
run_id: null
created_at: "2026-07-17T23:04:06.8719617+09:00"
created_by: "codex"
registration_state: pending-bootstrap
supersedes: null
---

# Domain Core Batch 2 완료보고

## 구현 범위

- 공통 `Diagnostic`과 `Evaluation` 모델, 안정적 diagnostic 정렬
- typed Task status와 lifecycle transition
- lifecycle과 분리된 blocker metadata 및 Run 시작 정책
- Task, dependency, 명시적 Gate 조건을 포함하는 `WorkspaceGraph`
- terminal update, remove, add, 최종 graph 검증을 하나의 후보 복사본에서 수행하는 atomic dependency patch
- connect, disconnect, set/clear terminal convenience operation의 patch 수렴
- root, leaf, dangling 변화가 포함된 preview diff
- Task status, Run kind, diagnostic code의 V1 JSON 계약 일치 테스트

PostgreSQL, HTTP/MCP, Event persistence, HumanApprovalAttestation, Gate pass, Run lease, Viewer는 계획대로 이번 배치에 포함하지 않았다.

## 변경 파일

`server/internal/domain/` 아래에 다음 구현과 테스트를 추가·정리했다.

- `diagnostic.go`
- `task.go`, `task_test.go`
- `workspace_graph.go`
- `dependency_patch.go`, `dependency_patch_test.go`
- `path_projection.go`
- `run_policy.go`, `run_policy_test.go`
- `contracts_test.go`
- 기존 dependency 및 Gate 테스트 호환 정리

## 검증 결과

- `gofmt`: 통과
- `go test -count=1 ./...`: 통과
- `go vet ./...`: 통과
- `npm test -- --reporter=dot`: 3 files, 9 tests 통과
- `npm run build`: 통과
- `go test -race ./...`: 로컬 Windows 환경에 C compiler `gcc`가 없어 실행 불가

프런트 빌드에는 500 kB를 넘는 기존 번들 경고가 있으나 빌드는 성공했고 이번 Go domain 변경과 직접 관련이 없다.

## 독립 리뷰와 반영

독립 리뷰는 처음에 다음 3건을 보고했다.

1. 호환 생성자가 초기 graph Evaluation을 폐기함
2. blocked Task discard 후 blocker metadata가 잔존함
3. 계약 밖 Run kind가 허용됨

세 항목을 모두 코드와 회귀 테스트에 반영했다. 동일 리뷰어의 재검토 결과 모두 해소됐고 새 기능 회귀는 발견되지 않았다. 상세 대응은 `review-response-01.md`에 기록했다.

## 잔여 위험과 기술부채

- race 테스트가 도구 체인 제약으로 실행되지 않았다. 현재 aggregate는 내부 mutex를 갖지 않으므로 후속 command service와 transaction 계층에서 Workspace 단일 writer를 보장해야 한다.
- `WorkspaceGraph`의 map 필드는 목표 구조에 맞춰 노출돼 있다. 후속 상위 계층이 직접 map을 변경하면 patch invariant를 우회할 수 있으므로 command service에서는 patch API만 사용해야 하며, 필요하면 후속 배치에서 읽기 전용 accessor로 캡슐화한다.
- `invalid_state_transition`은 현재 계약 안에서 unknown Run kind를 거부하기 위해 사용했다. transport 입력 진단을 더 세분화하려면 계약 변경을 먼저 수행해야 한다.

## 명세와 구현 사이에 남은 범위

이번 배치의 순수 도메인 목표는 구현됐다. V1 전체 명세 중 persistence, revision/idempotency, Event, approval attestation, Gate/Phase transition, Run lease/heartbeat는 의도적으로 다음 배치에 남아 있다.

## 다음 배치 제안

Gate readiness, Phase transition, HumanApprovalAttestation을 command service transaction과 연결하고, Workspace revision 및 Event 원자성을 구현한다.

Baley가 구현 품질을 인증했다고 주장하지 않는다. 위 평가는 구현 주체의 테스트와 독립 리뷰 결과를 구분해 기록한 것이다.
