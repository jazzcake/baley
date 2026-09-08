---
baley_record: 1
record_id: "6c83aaba-d611-4c6f-8f6f-d282d2461182"
task_id: 182
record_type: detailed-plan
run_id: "3d901b7e-a34c-4dbf-9ebe-8b64b5dc8aa6"
created_at: "2026-09-08T11:53:00+09:00"
created_by: "codex-operator"
supersedes: null
---

# Task #182 상세 계획 — 개발 의사결정 로그 DB 감사

## 목표

현재 Baley API가 사용하는 운영 PostgreSQL 데이터베이스를 직접 read-only SQL로 조사해 개발 의사결정 흔적의 실제 저장 범위, 연결 품질, 복구 한계를 정량화한다. 그 근거와 분리해 최소 decision-log 모델 및 단계적 도입안을 제안한다.

## 권한과 경계

- 사용자는 Baley 제품 오너·PM·사람 승인자다.
- 분석 근거는 운영 PostgreSQL 직접 접속에서 실행한 `BEGIN TRANSACTION READ ONLY`, `SELECT`, `ROLLBACK` 결과로 제한한다.
- Baley MCP는 Task #182의 Run/Task Record bookkeeping에만 사용하며 분석 근거로 사용하지 않는다.
- 접속 문자열, 비밀번호, token, 사람 식별 정보와 원문 hash는 terminal·SQL 파일·보고서에 노출하지 않는다.
- DDL, DML, migration, DB 설정, 서비스, 방화벽, Tailscale, 배포와 운영 branch를 변경하지 않는다.
- 이 Task는 조사와 설계 제안만 수행한다. schema/API/MCP/UI 구현, commit, push는 제외한다.

## 운영 DB 식별 기준

1. 현재 실행 중인 Baley API 컨테이너가 가리키는 DB host·database를 비밀값 없이 구조적으로 판별한다.
2. Bird View 격리 컨테이너와 포트, volume, database name이 분리됐는지 확인한다.
3. 대상 DB에서 read-only transaction 상태, 핵심 table, 기대 Workspace, Task #182와 현재 revision이 존재하는지 확인한다.
4. 이후 모든 분석 쿼리는 위에서 검증한 대상과 Workspace에만 실행한다.

## 조사 순서

1. `information_schema`와 PostgreSQL catalog에서 실제 table, column, PK/FK를 수집한다.
2. Event, command, Task, Run, Task Record, acceptance evidence, commit, Git observation, 승인·감사 계층의 row count와 시간 범위를 산출한다.
3. Task 기준 연결률과 missing/orphan linkage를 정량화한다.
4. Event payload key와 command/result key 분포를 집계해 이유·대안·선택·결과·supersedes의 복구 가능성을 평가한다.
5. DB row만 사용해 Task #179, #180, #181과 관련 Event·Run·Record·commit·evidence를 시간순으로 재구성한다.
6. 전체 기간에서 판단 변경을 나타내는 후보 Event를 찾고 오래된 대표 사례 1건을 같은 방식으로 재구성한다.
7. 확인된 사실과 설계상 추론을 분리해 최소 decision-log 모델, 기록 시점, append-only 의미, 검색·복구·보안·보존 경계를 제안한다.
8. backfill 가능성을 계층별로 평가하고 최소 구현 순서와 후속 Task 후보를 제안한다.

## 산출물과 검증

- `docs/analysis/development-decision-log-db-audit.md`
- `docs/analysis/development-decision-log-readonly.sql`
- 이 상세 계획과 `task-182-completion-report.md`
- SQL 파일은 연결정보 없이 `psql -v workspace_id=...` 방식으로 재현 가능하게 작성한다.
- 문서에 적은 핵심 수치와 사례는 최종 read-only 재실행 결과와 대조한다.
- `git diff --check`와 변경 파일 목록으로 문서 품질 및 범위 준수를 확인한다.

## 초기 decision journal

| 시각(KST) | 판단 | 근거 | 영향 |
| --- | --- | --- | --- |
| 2026-09-08 11:45 | hosted-pilot 문서의 원격 DB를 운영 대상으로 가정하지 않는다. | `lucy`의 Baley service와 `devhub`의 PostgreSQL service가 모두 inactive였다. | 현재 실행 중인 로컬 API의 실제 DB 연결을 추적했다. |
| 2026-09-08 11:48 | `local-dev-postgres/baley`를 본 감사의 운영 대상으로 고정한다. | 실행 중인 `baley-api-1`의 DB URL은 main 컨테이너의 `baley` DB를 가리켰고, Bird View는 별도 컨테이너·포트·volume·`baley_v2_test` DB였다. | 이후 분석 SQL을 main DB와 대상 Workspace로 한정한다. |
| 2026-09-08 11:50 | 문서 schema가 아니라 catalog 결과를 기준으로 쿼리를 작성한다. | 운영 `workspaces.id`와 Event actor column명이 문서 예시와 달라 초기 join이 실패했다. | `information_schema` 선행 조사 후 확인된 column만 사용한다. |
