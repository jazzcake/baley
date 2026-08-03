# Adoption Pilot 운영 키트

이 문서는 기존 Git 프로젝트 하나를 Baley에 연결해 실제 Pilot을 운영하는
최소 절차의 정본이다. 실행 orchestration은
`.agents/skills/baley-adopt-project`를 사용하고, Task/Run/Record 및 사람
승인 규칙은 `.agents/skills/baley-manage-work`를 따른다.

## 지원되는 시작 경로

현재 runtime에서 새 Workspace를 만드는 정상 경로는 Viewer의 Workspace
메뉴다. 생성한 계정은 Owner로 결속된다. `project.bootstrap`은 domain
contract와 안전한 local planner는 있지만 public application/HTTP/MCP
execute transport가 아직 없으므로 직접 SQL로 흉내 내지 않는다.

Pilot 시작 순서는 다음과 같다.

1. Owner가 Viewer에서 Workspace를 생성한다.
2. Owner가 Participant 또는 Workspace-scoped Operator credential을
   준비한다.
3. Operator가 fresh read 후 typed `repository.register`로 repository와
   Task Record root를 연결한다.
4. 설치된 `baley-project-init` CLI로 `baley.yaml`, `.rgignore`,
   `.baley-init-state.json`, README, Task Record templates를 계획한다.
5. `create`와 검증된 `.rgignore`의 `merge`만 적용한다. `conflict`가 하나라도
   있으면 덮어쓰지 않고 중단한다.
6. 재실행 결과가 전부 `keep`인지 확인하고, 서버의 Workspace/repository
   binding과 config를 대조한다.
7. 대조가 끝나면 retry 전용 `.baley-init-state.json`을 삭제한다.

CLI 입력 JSON은 server/Workspace/repository/actor binding과
`"bootstrapCompleted": true`를 포함한다. 먼저 preview하고 conflict가
없을 때만 같은 입력으로 적용한다.

```powershell
baley-project-init --project-root . --input adoption-input.json
baley-project-init --project-root . --input adoption-input.json --apply
baley-project-init --project-root . --input adoption-input.json
```

마지막 실행의 모든 file action이 `keep`이어야 한다.

복원 state가 존재할 때 caller가 제공한 server, Workspace, repository,
actor 값은 state에 fingerprint로 묶인 값과 정확히 같아야 한다. 값이
다르면 새 plan을 만들지 않고 실패한다.

## 기본 운영 흐름

```text
lane Backlog
  → Phase를 정한 시점에 promote
  → Run start
  → 구현·검증·독립 리뷰
  → Record/Git evidence
  → delegated Task만 정책 충족 시 auto-confirm
  → human_required Task 및 Gate는 사람 결정 대기
```

Backlog 항목은 Lane에는 속하지만 생성 시 Phase를 가정하지 않는다. Run
중 session이 끊겼으면 새 작업을 만들기 전에 Run list와 read-only lane
brief를 fresh read한다. 아직 running이면 동일 client Run ID와 동일 payload
재시도로 lease를 복원하고, interrupted이면 기존 Run을 고치지 않고 새
Run으로 이어간다.

Record/Git mismatch는 자동 수정하지 않는다. lane brief는 mismatch를
보여주기만 하며 Workspace revision, Event, command를 바꾸지 않아야 한다.
수정이 필요하면 명시적 command와 새 Event/Record를 남긴다.

## 승인 경계

- delegated technical Task: immutable assignment와 typed evidence profile을
  충족한 경우에만 Task 자체를 auto-confirm할 수 있다.
- human_required Task: implemented에서 사람 확인을 기다린다.
- Gate pass, active Gate 조건 변경, Lane close/discard, Workspace close:
  fresh preview 뒤 authenticated human approval이 항상 필요하다.

여러 implemented Task의 확인은 같은 outcome을 묶어 한 번 질문할 수 있지만,
서버에서는 각 command를 fresh preview/execute loop로 순서대로 처리한다.
Gate pass는 Task 확인 뒤 새 revision으로 다시 preview하는 별도 결정이다.

## PilotMeasurement

Pilot sample은 `pilot-measurement` Task Record로 append-only 저장한다.
project-init이 만드는 `_templates/pilot-measurement.md`를 복사해 JSON
payload를 채운다.

```powershell
python <installed-skill-directory>/scripts/validate_pilot_measurement.py `
  task-records/<task>/pilot-measurement-01.md
```

필수 내용은 sample/session, 시작·종료 시각, Workspace revision, actor,
후보와 채택 후보, 거절 이유, evidence reference, mismatch key, correction
Event, Gate reference, conversation reference, 사람 판단 turn 수,
baseline/treatment 구분이다. 등록된 sample은 수정하지 않으며, correction은
새 Record로 만들고 correction Event IDs를 기록한다.

## Secret 규칙

Password, Agent token, Run lease token은 config, Record, log,
command argument에 저장하지 않는다. idempotency key는 command table에
원문으로 남을 수 있으므로 secret을 재사용하지 않는다.

## Enablement acceptance

격리된 test database와 임시 repository에서 다음을 한 번에 확인한다:
bootstrap/adoption, Backlog promote, Gate condition/entry와 G# lookup, Run
interruption/recovery, Record/Git mismatch read-only, delegated auto-confirm,
human authority boundary, mutation-attempt redaction, PilotMeasurement.

실행 entry point:

```powershell
./scripts/run-embedding-enablement-acceptance.ps1
```

이 검증은 single-repository Pilot 범위다. live Pilot DB cutover와 Gate passage는
포함하지 않는다.
