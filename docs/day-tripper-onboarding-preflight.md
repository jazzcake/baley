---
type: runbook
status: active
scope: local-day-tripper-onboarding-preflight
target_task: "#125"
---

# Day Tripper 온보딩 직전 Preflight

이 문서는 Day Tripper 데이터를 Baley에 등록하기 전에 끝내야 하는 최소 준비만 다룬다.
Hosted 배포, Google 로그인, 외부 사용자 초대와 #125의 실제 Lane/Backlog/Task 생성은
범위 밖이다.

## 필수 조건

1. `scripts/local-pilot-runtime.ps1`이 현재 Git commit의 API와 Viewer를 단일 관리
   runtime으로 실행한다.
2. PostgreSQL `54329`는 `127.0.0.1`에만 publish되고 schema version 16으로 ready다.
3. 현재 local Pilot DB의 logical backup을 별도 빈 DB에 복원하고 Workspace, Task,
   Event 수와 schema version이 원본과 일치한다.
4. Workspace-scoped Agent token이 operator capability만 가지며 Baley MCP가 인증된 read를
   수행할 수 있다.
5. fresh Baley read에서 #124가 `confirmed`, G#4가 `passed`, active Phase가
   `embedding-pilot`인지 확인한다.
6. Owner가 로그인하고 Baley Pilot graph가 정상적으로 표시되는지 확인한다.
7. repository worktree가 clean이고 전체 Go/Viewer acceptance가 통과한 commit을 사용한다.

## 표준 명령

```powershell
# 기존 수동 Baley API/Viewer를 먼저 명시적으로 종료한 뒤
.\scripts\local-pilot-runtime.ps1 start

$backup = .\scripts\local-pilot-db.ps1 backup
.\scripts\local-pilot-db.ps1 verify $backup

# Owner password는 화면에 표시되지 않으며 raw Agent token도 출력하지 않는다.
.\scripts\prepare-local-pilot-agent.ps1

.\scripts\test-local-pilot-preflight.ps1
```

API와 Viewer만 종료할 때는 다음을 사용한다. PostgreSQL과 data volume은 유지된다.

```powershell
.\scripts\local-pilot-runtime.ps1 stop
```

backup은 `.tmp/local-pilot/backups`에 생성되며 Git에 포함하지 않는다. 검증은
`baley_restore_verify_*` 이름의 고유 disposable DB만 생성·삭제하고 `baley` 원본 DB를
변경하지 않는다.

## 사람 확인 항목

자동 preflight는 DB 직접 조회로 Task/Gate 의미를 우회하지 않고 Workspace-scoped Agent
token으로 Baley HTTP read contract를 사용해 다음을 확인한다.

- #124 — Embedding Enablement 수용 검증: `confirmed`
- G#4 — Embedding Enablement → Embedding Pilot: `passed`
- active Phase: `embedding-pilot`
- Owner 계정: Workspace 관리와 사람 승인 가능
- Agent token: 사람 승인과 Workspace administration 불가

Participant 계정과 다중 Workspace의 실제 browser 검증은 다중 사용자 Pilot 전에는
필수지만, 단일 사용자 Day Tripper #125 온보딩의 선행조건으로 계정을 새로 만들지는 않는다.

모든 항목이 통과한 뒤에만 #125 Run을 시작하고 Day Tripper의 실제 구조를 등록한다.

## 2026-08-03 준비 결과

- Workspace revision: 591
- active Phase: `embedding-pilot`
- #124: `confirmed`
- G#4: `passed`
- #125: `pending` — 아직 Run이나 구조 등록을 시작하지 않음
- local PostgreSQL: healthy, `127.0.0.1:54329` only
- backup restore: Workspace 1, Task 30, Event 438, schema 16 일치
- managed runtime: API `127.0.0.1:8080`, Viewer `127.0.0.1:5174`
- Agent token: operator scope authenticated read 통과, raw token은 User environment에만 저장

repository 변경을 commit하고 해당 commit으로 runtime을 재시작하면 자동 preflight의
`clean-worktree`와 `runtime-current-commit` 조건이 최종 통과한다. Owner는 Viewer에서 graph가
표시되는지만 한 번 확인한다.
