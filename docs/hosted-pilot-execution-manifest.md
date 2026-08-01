---
type: execution-manifest
status: superseded
scope: hosted-pilot
superseded_by: docs/hosted-pilot-simple-execution-manifest.md
related:
  - docs/baley-roadmap.md
  - docs/hosted-pilot-deployment-plan.md
  - docs/multi-user-collaboration-plan.md
  - docs/operations-quality-plan.md
---

# Hosted Pilot 실행 manifest

> 이 manifest는 backend를 devhub에 두는 이전 topology를 전제로 한다. 2026-08-01
> Owner가 `lucy = Web/Application`, `devhub = tailnet PostgreSQL` 단순 구조를
> 확정했으므로 `docs/hosted-pilot-simple-execution-manifest.md`로 대체한다.

## 1. 목표와 실제 host 배치

목표는 로컬에서 동작하는 Baley를 실제 두 서버에 안전하게 연결해 소규모 초대형
Pilot으로 운영하는 것이다.

| 논리 host | 실제 대상 | 책임 |
| --- | --- | --- |
| `devhub` | 준비된 4 core, 16 GB 서버 | Baley API, PostgreSQL, migration, backup staging, backend 관측 |
| `web-front` | Owner가 접속 주소를 별도 제공한 웹 전용 서버 | 정적 Viewer, public ingress, `/api/*` private reverse proxy |

`web-front`는 실제 hostname을 받기 전까지만 쓰는 논리명이다. PostgreSQL과
`baley-server`의 loopback port는 public Internet에 열지 않는다.

### 확인된 `web-front` baseline

- Lucy English의 `learning_english-app-1` container만 실행 중이며 약 4개월간
  재시작 없이 운영됐다.
- 443/3001은 `docker-proxy`를 통해 Lucy container로 연결되고, 별도 Nginx는 없다.
- Lucy의 Go application이 Cloudflare Origin certificate로 TLS를 직접 처리한다.
- host listener는 22, 443, 3001이다.
- disk는 39 GB 중 약 7.2 GB를 사용하며 load는 거의 없다.
- memory는 약 914 MB이고 약 516 MB가 available하다.

따라서 `web-front`에는 정적 Viewer와 경량 ingress만 배치한다. 첫 배포에서는 Lucy의
443 listener와 container를 변경하지 않는다. 공개 IP는 Git에 기록하지 않는다.

## 2. 실행 원칙

- 실제 host의 기존 서비스를 발견 단계에서 변경하지 않는다.
- secret, password, DB dump와 token은 Git, Task Record와 채팅에 넣지 않는다.
- production artifact와 deployment template은 개발용 `docker-compose.yml`과 분리한다.
- DB migration 전 backup, deploy 후 smoke test, 실패 시 application rollback 경로를
  항상 한 묶음으로 구현한다.
- 사람 확인은 domain/product 판단, 실제 production exposure, destructive restore와
  Pilot 사용자 초대 경계에 둔다. 일반 코드 구현·테스트·독립 리뷰는 Agent가 완료한다.

## 3. Wave와 Task 후보

아래 `HP-*`는 manifest용 참조이며 Baley public Task ID가 아니다. typed
`task.create` 실행 뒤 발급된 `#<publicId>`를 별도로 기록한다.

### Wave 0 — 실제 환경 inventory

#### HP-01 — 두 host의 read-only inventory

- `devhub`와 `web-front`의 OS/architecture, SSH alias, firewall, disk, Docker 또는
  service manager, 기존 reverse proxy를 읽기 전용으로 조사한다.
- 두 host 사이의 private network/tunnel과 사용할 public domain/DNS를 확인한다.
- 기존 서비스 port, volume과 배포 경로를 건드리지 않는 격리 경계를 기록한다.
- predecessor: 없음. 이 Hosted Pilot Track의 명시적 root다.
- human confirm: 불필요. 실제 변경 없는 조사 결과를 Owner가 검토한다.
- status: `web-front`의 Owner 제공 baseline은 수집 완료. SSH read-only 검증과
  `devhub` inventory가 남아 있다.

#### HP-02 — production topology 결정 기록

- HP-01 결과로 host service/container 방식, private link, reverse proxy, domain과
  certificate 방식을 확정한다.
- predecessor: HP-01
- human confirm: 필요. 실제 공개 topology와 운영 경계를 Owner가 결정한다.
- front decision: Lucy의 443을 유지하는 별도 Cloudflare ingress/tunnel 방식과,
  rollback을 준비한 shared gateway 전환 방식을 HP-01 증거로 비교한다.

### Wave 1 — 배포 가능한 제품 baseline

#### HP-03 — backend/Viewer production artifact

- versioned backend와 Viewer artifact, non-secret production deployment template,
  build metadata를 구현한다.
- predecessor: HP-02
- human confirm: 불필요. 구현·테스트·독립 리뷰로 delegated acceptance 가능하다.

#### HP-04 — secret file과 production config validation

- DB credential, lease secret과 향후 Google 설정을 image/Git 밖에서 주입하고,
  누락·legacy auth·insecure cookie 설정을 startup에서 거부한다.
- predecessor: HP-02
- human confirm: 불필요. 보안 독립 리뷰는 필수다.

#### HP-05 — readiness, version과 HTTP 운영 경계

- `/readyz`, schema/build version, timeout, body limit, request ID와 credential
  redaction이 적용된 structured access log를 구현한다.
- predecessor: HP-03, HP-04
- human confirm: 불필요. 보안·운영 리뷰는 필수다.

### Wave 2 — 데이터 이동과 devhub staging

#### HP-06 — 로컬 Pilot DB export/import

- Docker PostgreSQL의 custom-format dump, SHA-256 검증, 대상 DB 안전 백업과
  명시적 교체 guard를 구현한다.
- 기존 `BALEY_LEASE_TOKEN_SECRET`은 dump와 분리해 안전하게 이전한다.
- predecessor: HP-04
- human confirm: 코드 완료 확인은 불필요할 수 있으나 실제 대상 DB 교체는 매번 필요하다.

#### HP-07 — devhub backend staging 배포

- `devhub`에 격리된 PostgreSQL과 backend를 배치하고 migration, restore,
  `/healthz`, `/readyz`, login API와 rollback을 검증한다.
- predecessor: HP-05, HP-06
- human confirm: `devhub`에 최초 mutation을 시작하기 전에 필요하다. 이후 승인된
  배포 절차 안의 반복 가능한 update는 runbook 정책을 따른다.

### Wave 3 — web-front 공개 경로

#### HP-08 — private front/backend 연결

- `web-front`에서만 `devhub` gateway에 접근 가능한 encrypted private link와
  firewall 경계를 구성한다.
- predecessor: HP-07
- human confirm: 실제 network/firewall 변경 전에 필요하다.

#### HP-09 — Viewer, reverse proxy, domain과 TLS

- `web-front`에 Viewer를 배포하고 same-origin `/api/*` proxy, domain, TLS,
  security header와 maintenance response를 구성한다.
- predecessor: HP-08
- human confirm: public DNS 전환과 외부 공개 직전에 필요하다.

### Wave 4 — Hosted 사용자 경계

#### HP-10 — Site Operator와 Workspace Owner 권한 분리

- global Account 복구와 Workspace 제품 권한을 분리하고 Owner가 다른 사용자의
  global credential을 변경하지 못하게 한다.
- predecessor: HP-05
- human confirm: 권한 정책 수용 확인 필요. 구현·테스트·보안 리뷰 후 판단한다.

#### HP-11 — Google identity와 one-time invite

- server-side Google ID token 검증, GIS CSRF, external identity, pending login과
  one-time Workspace invite를 구현한다.
- predecessor: HP-09, HP-10
- human confirm: Google production client와 초대 정책 수용 확인 필요하다.

### Wave 5 — 운영 품질과 Pilot 개시

#### HP-12 — backup/restore, 관측과 alert

- daily encrypted off-host backup, checksum, retention, isolated restore drill,
  metrics와 readiness/disk/backup-age alert를 구성한다.
- predecessor: HP-07
- human confirm: backup 목적지와 retention 결정에 필요하다.

#### HP-13 — Hosted staging 종합 수용

- 두 Account·두 Workspace 격리, login/logout, Workspace 전환, Agent token,
  approval grant, backup restore와 application rollback을 실제 host에서 검증한다.
- predecessor: HP-09, HP-11, HP-12
- human confirm: 필요. Pilot 공개 Gate의 근거다.

#### HP-14 — 초대형 Pilot 개시

- 승인된 소수 사용자에게 invite를 전달하고 운영 지표와 incident 채널을 연다.
- predecessor: HP-13
- human confirm: 필요. 실제 외부 사용자 초대 결정이다.
- terminal intent: Hosted Pilot 운영 측정으로 이어지는 의도된 leaf다.

## 4. 첫 실행 묶음

첫 구현 세션은 HP-01만 수행한다. read-only inventory 결과로 HP-02의 선택지가
구체화되기 전에는 production Dockerfile, proxy 제품 또는 tunnel을 추측해 고정하지 않는다.

HP-01에 필요한 사용자 입력은 secret이 아니라 다음 접속 식별 정보뿐이다.

- `devhub` SSH alias 또는 접속 방법
- 웹 전용 서버의 SSH 사용자/alias 또는 이미 설정된 접속 방법
- 두 서버에서 read-only inventory 명령 실행 허용 여부

접속 credential 자체는 채팅에 붙이지 않고 이미 구성된 SSH agent/config 또는 별도
credential manager를 사용한다.
