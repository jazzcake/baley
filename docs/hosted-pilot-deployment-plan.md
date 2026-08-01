---
type: implementation-plan
status: superseded
scope: hosted-pilot
superseded_by: docs/hosted-pilot-simple-server-architecture.md
related:
  - docs/account-workspace-access-contract.md
  - docs/account-workspace-access-operations.md
  - docs/multi-user-collaboration-plan.md
  - docs/operations-quality-plan.md
---

# Hosted Baley Pilot 서비스 서버·배포 기획

> 이 문서는 초기 `devhub=API+DB` 배치 검토 기록이다. 이후 실제 host inventory와
> Owner 결정으로 `lucy=Viewer+API`, `devhub=shared PostgreSQL` 구조가 채택됐으며,
> 현재 정본은 `hosted-pilot-simple-server-architecture.md`다. Hosted 배포 자체는
> 2026-08-02 로컬 기능 마감 결정에 따라 보류됐다.

## 1. 결정된 전제

- Baley는 사용자가 이미 외부 서비스용으로 운영하는 두 서버에 실제 배포한다.
- `devhub`는 backend host다. 4 core, 16 GB RAM 기준으로 Baley API,
  PostgreSQL과 local backup staging을 맡는다.
- 대기 중인 웹 전용 서버는 front host다. 실제 hostname이 확정되기 전까지
  `web-front`라는 논리명을 사용하며 정적 Viewer, TLS와 공개 진입점을 맡는다.
- 두 host는 기존 서비스와 격리된 Baley process/container, network, volume과
  secret 범위를 사용한다.
- 초기 사용자는 지인·친구 중심의 소규모 초대형 Pilot이다.
- 일반 사용자의 기본 사람 인증은 Google Identity Services 로그인으로 한다.
- local password 인증은 최초 Site Operator bootstrap과 비상 복구 경계로 유지한다.
- 초기에는 단일 backend instance와 단일 PostgreSQL primary를 사용한다.
- Kubernetes, active-active, 자동 failover, public sign-up은 초기 범위가 아니다.
- 서버 OS와 기존 reverse proxy 제품은 아직 확정하지 않았다. 구현 예시는 Linux와
  container/system service를 기준으로 하되 기존 홈프로젝트 운영 방식에 맞춘다.

## 2. 목표

1. 브라우저에는 하나의 HTTPS origin만 노출한다.
2. Baley API와 PostgreSQL을 public Internet에 직접 노출하지 않는다.
3. 다른 홈프로젝트와 process, network, DB role, volume, secret, backup을 분리한다.
4. 새 버전을 반복 배포하고 실패 시 application을 되돌릴 수 있다.
5. 운영 DB를 자동 백업하고 실제 다른 DB로 복원할 수 있다.
6. 로그인, Workspace 격리, Agent token과 사람 승인 경계를 production mode로 유지한다.

## 3. 권장 topology

```text
Browser
  |
  | HTTPS: https://baley.example
  v
web-front (웹 전용 서버)
  |- /            -> versioned static Viewer files
  `- /api/*       -> private encrypted link로 backend gateway에 reverse proxy
                          |
                          v
devhub gateway
  `- 127.0.0.1:8080 -> baley-server
                          |
                          v
                    PostgreSQL localhost/private container network
```

front와 backend 사이에는 기존 사설망, WireGuard/Tailscale 같은 tunnel, 또는 상호
인증된 TLS 연결 중 하나를 사용한다. backend public firewall은 front server에서 오는
gateway traffic만 허용한다. PostgreSQL 5432와 Baley의 loopback 8080은 public interface에
publish하지 않는다.

실제 배포 연결은 다음 순서로 연다.

1. `devhub` 내부에서 PostgreSQL, migration, `baley-server`, `/healthz`와 `/readyz`를
   먼저 검증한다.
2. `web-front`와 `devhub` 사이의 private link를 구성하고 backend gateway만
   `web-front`에서 접근 가능하게 한다.
3. `web-front`에 versioned Viewer artifact와 `/api/*` reverse proxy를 배치한다.
4. production domain과 TLS를 연결한 뒤 login, Workspace 전환과 logout을 검증한다.
5. backup/restore drill과 rollback을 통과한 뒤 초대형 Pilot 사용자를 받는다.

3단계 전에는 `devhub`의 Baley API와 PostgreSQL을 public Internet에 노출하지 않는다.

### 3.1 `web-front` 확인 현황과 보호 경계

2026-08-01 Owner inventory 기준으로 `web-front`에는 Lucy English 한 서비스만
실행 중이다.

| 항목 | 확인 내용 |
| --- | --- |
| container | `learning_english-app-1` (`learning_english-app` image) |
| service age | 약 4개월 연속 실행 |
| public listener | 443 HTTPS, Cloudflare Origin certificate |
| additional listener | 3001 HTTP |
| ingress 구조 | 443/3001 모두 `docker-proxy`가 container로 전달 |
| application TLS | 별도 Nginx 없이 Lucy의 Go process가 직접 처리 |
| host listener | 22, 443, 3001 |
| disk | 39 GB 중 약 7.2 GB 사용 |
| memory | 약 914 MB, 약 516 MB available |
| load | 거의 유휴 |

공개 IP와 접속 credential은 repository에 기록하지 않고 SSH config 또는 별도 운영
inventory에서 관리한다.

이 환경에서 확정된 보호 경계는 다음과 같다.

- Baley Viewer를 build하는 작업은 다른 host/CI에서 수행하고 `web-front`에는 정적
  artifact만 배치한다.
- `web-front`에 Baley API나 PostgreSQL을 배치하지 않는다.
- Lucy의 443 listener, certificate와 container를 첫 Baley 배포에서 변경하지 않는다.
- Baley용 별도 public hostname은 Lucy와 충돌하지 않는 ingress를 사용한다. 기존
  Cloudflare 구성을 이용한 별도 tunnel 또는 shared front gateway가 후보지만 HP-02
  topology 결정 전에는 하나로 고정하지 않는다.
- shared gateway를 선택해 443을 이관해야 한다면 Lucy의 listener 변경, health check와
  즉시 rollback을 별도 사람 승인 작업으로 다룬다.

메모리 여유가 크지는 않으므로 `web-front`의 Baley 구성은 정적 파일 serving과
경량 ingress로 제한하고 실제 사용량을 관측한다.

동일-origin 구성을 우선한다.

- Viewer build: `VITE_BALEY_API_URL=https://baley.example/api`
- API runtime: `BALEY_VIEWER_ORIGINS=https://baley.example`
- front proxy는 `/api` prefix를 제거한 뒤 backend의 `/v1`, `/healthz`, 향후 `/readyz`로
  전달한다.
- browser Session과 CSRF cookie는 `baley.example`의 `__Host-` Secure Cookie로 유지한다.

현재 `baley-server`는 loopback bind만 허용하므로 이 경계를 완화하지 않는다. backend
gateway가 private link와 loopback application 사이를 연결한다. 사용 중인 web server가
Caddy가 아니어도 동일한 경계로 Nginx 또는 기존 gateway를 사용할 수 있다.

현재 TCP loopback 제한은 host service와 host gateway 조합에는 바로 맞지만, 서로 다른
container의 gateway가 app container에 접근하는 구성에는 맞지 않는다. production 구현에서
다음 중 하나를 명시적으로 선택한다.

- host service 또는 같은 network namespace에서 loopback proxy 사용
- gateway와 공유하는 Unix socket 지원 추가
- 지정된 private/container interface만 허용하는 별도 bind 정책 추가

편의를 위해 `0.0.0.0:8080`을 그대로 허용하거나 host에 publish하는 방식은 선택하지 않는다.

## 4. 현재 repository와 production gap

현재 구현에서 재사용할 수 있는 항목:

- production에서 `BALEY_AUTH_MODE=enforced` 강제
- Secure Cookie, exact Origin, CSRF 검증
- Argon2id password와 hash-only Session/Agent token 저장
- Workspace membership default-deny와 cross-Workspace 격리
- graceful shutdown과 `/healthz`
- migration CLI와 PostgreSQL integration 검증

배포 전 새로 필요한 항목:

- production backend와 Viewer용 multi-stage Dockerfile 또는 동등한 versioned artifact
- 개발용 `docker-compose.yml`과 분리된 production deployment template
- hard-coded PostgreSQL password와 host port publish 제거
- secret file 또는 service credential에서 설정을 읽는 경로
- DB 연결과 migration 상태까지 확인하는 `/readyz`
- backup, restore verification, deploy, rollback script/runbook
- front/backend version을 함께 확인할 build metadata endpoint 또는 response header
- request timeout, body limit, structured access log와 request ID
- Google ID token의 server-side 검증, external identity binding과 invite 연결
- Google Identity Services용 CSP/COOP와 exact production login endpoint 설정

현재 `docker-compose.yml`은 개발 전용이다. `POSTGRES_PASSWORD=baley`, host port 54329
publish와 source-tree migration 경로를 그대로 production에 사용하지 않는다.

## 5. backend 자원 기준

4 core, 16 GB는 소규모 Pilot에서 Baley API와 PostgreSQL을 한 머신에 두기에 충분한
출발점이다. 다른 홈프로젝트와 공유되므로 고정 성능을 가정하지 않고 다음 경계를 둔다.

- Baley app과 PostgreSQL은 별도 service/container와 전용 network를 사용한다.
- PostgreSQL data와 backup staging은 서로 다른 persistent path를 사용한다.
- 전체 메모리를 PostgreSQL에 할당하지 않고 OS, page cache, 다른 서비스와 장애 여유를
  남긴다. 실제 제한값은 배포 후 RSS, DB cache hit, connection 수를 보고 결정한다.
- app instance는 하나로 시작한다. 모든 process에 같은 lease secret을 넣는 다중 instance
  운영은 부하 근거가 생긴 뒤 검토한다.
- disk 사용량, inode, PostgreSQL volume과 backup age를 필수 관측값으로 둔다.

## 6. production configuration과 secret

필수 runtime 값:

```text
BALEY_ENV=production
BALEY_AUTH_MODE=enforced
BALEY_COOKIE_SECURE=true
BALEY_HTTP_ADDR=127.0.0.1:8080
BALEY_VIEWER_ORIGINS=https://baley.example
BALEY_DATABASE_URL=<secret reference>
BALEY_LEASE_TOKEN_SECRET=<stable secret reference>
BALEY_GOOGLE_CLIENT_ID=<Google web client ID>
BALEY_GOOGLE_LOGIN_ENABLED=true
```

secret 범위:

- PostgreSQL application/backup/migration credentials
- `BALEY_LEASE_TOKEN_SECRET`
- backup destination credential와 encryption key
- front/backend private-link credential
- 최초 Site Operator 또는 Owner bootstrap credential

Google web client ID는 public identifier지만 production configuration으로 관리한다. 로그인용
GIS ID token flow는 Google API access/refresh token을 요구하지 않으며 client secret도
저장하지 않는다. 향후 Google Drive 같은 API 접근이 필요해질 때만 별도의 OAuth
authorization과 secret 범위를 설계한다.

secret는 Git, image, Compose environment literal, Task Record, shell history와 로그에 넣지
않는다. Docker Compose를 사용하면 mounted secret file을 사용하고, Baley에는 `*_FILE`
또는 명시적 secret-file 설정을 추가한다. Docker의 Compose secrets도 secret를 container
file로 mount하는 방식을 제공한다.

lease secret 교체는 active Run token을 무효화할 수 있으므로 일반 배포와 분리된 rotation
절차로 다룬다.

## 7. 데이터와 backup

초기 목표는 `RPO <= 24시간`, 운영자 접근 확보 후 `RTO <= 4시간`이다.

1. 매일 `pg_dump -Fc` logical backup을 생성한다.
2. backup은 생성 직후 암호화하고 backend와 다른 저장소로 복사한다.
3. 최소 7개 daily와 4개 weekly backup을 유지한다.
4. backup job은 파일 생성만으로 성공 처리하지 않고 size, exit status, checksum과 age를
   기록한다.
5. 매월 빈 PostgreSQL instance에 `pg_restore`하고 migration version, Owner invariant,
   Workspace/Task/Event count와 login smoke test를 검증한다.
6. 배포 migration 직전 별도 backup을 남긴다.
7. 사용량과 데이터 가치가 커지면 base backup과 WAL archive/PITR을 추가한다.

`pg_dump`는 실행 시점의 일관된 snapshot을 만들고 custom format은 `pg_restore`를 통한
선택적 복원을 지원한다. PITR은 base backup과 연속 WAL archive가 필요하므로 초기 daily
backup과 구분해 단계적으로 도입한다.

## 8. 배포와 rollback 순서

1. CI에서 Go/React/PostgreSQL/acceptance 검증을 통과한다.
2. commit SHA와 build time을 포함한 immutable backend/Viewer artifact를 만든다.
3. 운영 DB의 migration 직전 backup과 복구 가능 상태를 확인한다.
4. migration job을 한 번만 실행한다.
5. backend를 교체하고 `/healthz`, `/readyz`, version을 확인한다.
6. test Account로 login, Workspace list와 graph read smoke test를 실행한다.
7. Viewer 정적 artifact를 원자적으로 교체한다.
8. 실제 browser에서 login, Workspace switch, logout을 확인한다.
9. 실패 시 이전 application artifact로 되돌린다. 이미 적용된 migration은 호환 가능한
   additive migration이면 유지하고, destructive DB downgrade를 자동 실행하지 않는다.

## 9. 구현 단위

1. production image와 non-secret deployment template
2. file-based secret loading과 production config validation
3. `/readyz`, build metadata, HTTP timeout와 structured request log
4. front same-origin proxy example과 backend private gateway example
5. migration/backup/restore/deploy/rollback commands
6. clean production-like DB의 account bootstrap과 multi-Workspace browser smoke test
7. Google login, invite consumption과 local Site Operator break-glass smoke test
8. 실제 두 서버에 staging 배포 후 production cutover

## 10. 수용 기준

- public scan에서 front의 443만 필요 범위로 노출된다.
- PostgreSQL과 Baley loopback port는 외부에서 연결되지 않는다.
- production startup이 legacy auth, insecure cookie, 누락 secret와 Owner invariant 위반을
  거부한다.
- 같은 origin에서 login, CSRF mutation, Workspace 전환과 logout이 동작한다.
- Google 로그인 성공 후에도 Baley 자체 Session Cookie만 사용하고 Google ID/access token을
  browser storage나 Baley Session으로 재사용하지 않는다.
- 배포 artifact의 commit SHA를 운영 상태에서 확인할 수 있다.
- 자동 backup이 다른 storage에 존재하고 실제 빈 DB 복원이 통과한다.
- 한 버전 전 application rollback과 incident 기록 절차가 재현된다.

## 11. 구현 전 Owner 입력

host 역할 배치는 확정됐다. 다음 값은 repository에서 추측하지 않고 실제 서버
inventory에서 읽거나 Owner가 제공한 값으로 확정한다.

- `devhub`의 실제 hostname과 두 host의 OS·CPU architecture
- 두 host의 SSH 접속 별칭과 배포 사용자가 가진 권한 범위
- `web-front`의 Docker/Cloudflare 설정을 읽을 수 있는 배포 사용자와 Lucy rollback 경로
- `devhub`에서 현재 사용하는 gateway와 process/container 관리자
- 두 서버 사이의 사설망 또는 tunnel 유무
- 사용할 domain과 DNS 관리 위치
- Google Cloud project, OAuth consent branding과 production web client ID
- `devhub`의 기존 PostgreSQL 사용 여부와 Baley 전용 instance/cluster 선택
- encrypted off-host backup 목적지
- 기존 홈서비스 monitoring·alert 채널

이 입력은 topology 원칙을 바꾸지 않지만 배포 template과 명령 형식을 결정한다.

## 12. 참고 기준

- [Caddy reverse proxy documentation](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy)
- [Docker Compose secrets](https://docs.docker.com/compose/how-tos/use-secrets/)
- [PostgreSQL 17 SQL dump](https://www.postgresql.org/docs/17/backup-dump.html)
- [PostgreSQL continuous archiving and PITR](https://www.postgresql.org/docs/current/continuous-archiving.html)
- [OWASP Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)
- [Google Identity Services overview](https://developers.google.com/identity/gsi/web/guides/overview)
- [Verify Google ID tokens server-side](https://developers.google.com/identity/gsi/web/guides/verify-google-id-token)
