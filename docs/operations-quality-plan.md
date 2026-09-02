---
type: implementation-plan
status: draft-for-owner-review
scope: hosted-operations-quality
related:
  - docs/hosted-pilot-deployment-plan.md
  - docs/multi-user-collaboration-plan.md
  - docs/recovery/2026-07-26-baley-db-incident.md
---

# Hosted Baley 운영 품질 기획

## 1. 서비스 등급과 초기 목표

초기 서비스는 지인·친구 대상의 private Pilot이다.

- 예상 Account: 5~20명
- 예상 동시 사용자: 5명 이하
- backend: 4 core, 16 GB 단일 머신
- database: 같은 backend의 단일 PostgreSQL primary
- formal SLA와 24시간 당직은 없음
- 데이터 유실 목표: `RPO <= 24시간`
- 운영자 접근 확보 후 복구 목표: `RTO <= 4시간`

이 숫자는 보장이 아니라 초기 engineering target이다. 실제 사용량, 장애와 복원 측정으로
조정한다.

## 2. 품질 원칙

1. 서비스가 떠 있는 것과 요청을 받을 준비가 된 것을 구분한다.
2. backup 성공은 restore로 검증한다.
3. 배포 artifact와 DB migration version을 항상 식별할 수 있어야 한다.
4. 운영 로그와 Baley domain/security Event의 목적을 구분한다.
5. secret·credential·승인문과 command raw payload는 관측 데이터에 남기지 않는다.
6. 장애 대응은 수동이어도 되지만 명령과 판단 순서는 runbook으로 재현 가능해야 한다.

## 3. health와 readiness

현재 `/healthz`는 Go process liveness만 반환한다. 다음을 분리한다.

- `/healthz`: process event loop가 응답하는지 확인. DB를 조회하지 않는다.
- `/readyz`: DB ping, migration version, 필수 secret load, enforced Owner invariant와 command
  service 초기화를 확인한다.
- version projection: commit SHA, build time, schema version과 runtime mode를 반환하되 secret과
  상세 infrastructure 정보는 제외한다.

front는 backend `/readyz` 실패 시 mutation traffic을 보내지 않고 일반적인 maintenance
응답을 표시한다. health endpoint는 인증 없이 사용할 수 있지만 상세 오류는 서버 로그에만
남긴다.

## 4. structured logging

JSON line 또는 기존 중앙 수집기가 안정적으로 파싱할 수 있는 구조를 사용한다.

필수 필드:

```text
timestamp, level, service, version, request_id,
method, route_template, status, duration_ms,
actor_kind, workspace_id, command_name, outcome, error_code
```

금지 필드:

- password와 password hash
- Cookie, Session, CSRF, invite, recovery, Agent와 approval token
- `BALEY_DATABASE_URL`, lease secret와 backup credential
- raw command payload, 승인문 전문과 예상 밖 SQL/error 원문

request log, security log와 Baley domain Event는 보존 목적이 다르므로 한 stream으로
합치지 않는다. 운영 로그는 rotation과 최대 disk 사용량을 설정한다. 로그 저장 실패나
disk full이 서비스 전체를 연쇄 중단시키지 않는지도 테스트한다.

## 5. metrics와 alert

최소 metrics:

- HTTP request 수, 4xx/5xx, route별 latency p50/p95
- active request와 timeout 수
- login 성공/실패/rate-limit 수
- Google login 성공/검증 실패/provider dependency failure 수
- Workspace revision conflict와 idempotent replay 수
- command outcome과 mutation-attempt outcome 수
- PostgreSQL connection pool 사용량과 query error
- Run lease sweep 실패·interruption 수
- process CPU/RSS, host load, disk/inode와 DB volume 크기
- backup 마지막 성공 시각, 파일 크기, checksum과 마지막 restore drill
- TLS certificate 만료까지 남은 일수

초기 alert:

- public endpoint 또는 `/readyz` 5분 연속 실패
- 5xx 비율 급증 또는 DB connection failure
- disk 80% 경고, 90% 긴급
- 마지막 성공 backup이 26시간을 넘김
- restore drill이 예정일을 넘김
- 인증 실패·rate-limit이 평시 기준을 크게 초과
- TLS certificate 만료 14일 이내

알림은 기존 홈서비스 운영 채널을 재사용하되, credential과 사용자 입력을 포함하지 않는다.

## 6. CI quality gate

모든 production 후보 commit에서 다음을 자동화한다.

1. `go test ./...`
2. `go vet ./...`
3. disposable PostgreSQL fresh migration과 upgrade migration
4. destructive integration DB safety regression
5. `npm test`, `npm run typecheck`, `npm run build`
6. login, 두 Workspace 생성·전환·재로그인·격리 browser E2E
7. Google verifier의 signature/audience/issuer/expiry/CSRF/invite replay 회귀 테스트
8. staging의 실제 Sign in with Google smoke test
9. Agent token의 사람 권한 차단과 브라우저 단발성 human grant E2E
10. `git diff --check`, secret scan과 dependency vulnerability scan
11. production image build와 non-root/runtime config smoke test
12. backup artifact 생성과 주기적인 restore workflow

실패한 품질 gate를 수동으로 무시할 때는 이유와 만료 시점을 release evidence에 남긴다.

## 7. 성능과 용량

Pilot 전 synthetic baseline을 기록한다.

- 5명의 동시 login과 Workspace graph polling
- 5개 Workspace에서 독립 command mutation
- 큰 Workspace fixture의 graph payload size와 Viewer render time
- Argon2id login burst에서 CPU와 memory, rate limiter 동작
- backup 중 API latency와 PostgreSQL I/O
- Run lease sweep과 일반 command가 함께 실행될 때의 lock wait

초기 목적은 높은 TPS가 아니라 4 core/16 GB 공유 머신에서 다른 서비스에 영향을 주지
않는 자원 경계를 찾는 것이다. 결과로 connection pool, polling interval, container limit와
backup 시간을 조정한다.

## 8. 보안 운영

- 전체 browser session에서 HTTPS와 HSTS를 사용한다.
- Google Identity Services에 필요한 최소 CSP `script-src`, `frame-src`, `connect-src`만
  허용하고 production domain/redirect endpoint를 Google Cloud 설정과 정확히 맞춘다.
- dependency와 base image를 정기 갱신하고 긴급 보안 수정 경로를 둔다.
- production DB role, migration role, backup role과 test role을 분리한다.
- PostgreSQL은 public port를 열지 않는다.
- host/container는 최소 권한과 read-only filesystem을 우선하며 필요한 volume만 쓴다.
- secret rotation은 DB credential, lease secret, Agent token, backup credential별 runbook을
  갖는다.
- 사용자 권한 변경, Site Operator 복구와 secret rotation은 security audit 대상이다.
- 분기별로 cross-Workspace 접근과 credential redaction 회귀를 다시 실행한다.

## 9. backup·restore 품질

- daily custom-format logical dump와 off-host encrypted copy
- backup retention 자동 정리와 deletion guard
- checksum, PostgreSQL version, migration version과 생성 commit metadata 보존
- 월 1회 isolated restore와 application smoke test
- 분기 1회 backend machine 상실을 가정한 새 머신 복구 연습
- 복구 중 production DB를 test fixture로 사용하지 못하게 role, database name과 network를
  분리
- 데이터 증가 또는 RPO 요구 강화 시 base backup + WAL archive/PITR 추가

복원 성공 보고에는 단순 command exit code가 아니라 login, Workspace count, selected graph,
Task/Event count와 latest migration 확인을 포함한다.

## 10. 배포·장애 runbook

필수 runbook:

- 정상 deploy와 migration
- application rollback
- failed migration 대응
- DB connection exhaustion
- disk full과 log growth
- expired TLS certificate
- compromised Session/Agent/invite/recovery credential
- Google login provider 장애, 잘못된 key cache와 identity-link 사고
- lost front server와 lost backend server
- PostgreSQL logical restore와 향후 PITR
- 운영 DB 오사용 또는 destructive test 사고

각 runbook은 탐지 신호, 즉시 완화, 데이터 보존, 복구 명령, 검증, 사용자 공지 여부와
사후 기록 위치를 포함한다.

## 11. 구현 단위

1. `/readyz`, build/schema version과 HTTP timeout
2. request ID, structured access log와 redaction test
3. metrics endpoint와 host/service dashboard
4. backup freshness, disk, readiness와 certificate alert
5. CI PostgreSQL/browser/image/security gate
6. Google login verifier/key cache metrics와 staging smoke test
7. scheduled backup/restore verification
8. load baseline과 resource limit 조정
9. incident/deploy/rollback runbook drill

## 12. 수용 기준

- liveness와 readiness 실패를 구별해 감지한다.
- 운영 중인 commit과 DB schema version을 즉시 식별한다.
- raw credential이 log, metric, Event와 error response에 남지 않는다.
- 26시간 이상 backup이 없으면 알림이 발생한다.
- 다른 머신의 빈 DB로 최신 backup을 복원하고 login/Workspace smoke test가 통과한다.
- 두 Workspace와 두 Account의 실제 browser 격리 E2E가 CI에서 반복 실행된다.
- Google provider 장애 중 기존 Baley Session은 정책상 만료될 때까지 유지되고 신규 Google
  login 실패는 명확히 관측된다.
- 이전 application artifact rollback과 주요 장애 runbook을 staging에서 연습한다.

## 13. 참고 기준

- [OWASP Logging Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html)
- [OWASP Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)
- [PostgreSQL backup and restore](https://www.postgresql.org/docs/17/backup.html)
- [PostgreSQL continuous archiving and PITR](https://www.postgresql.org/docs/current/continuous-archiving.html)
