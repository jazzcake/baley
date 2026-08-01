---
type: execution-manifest
status: deferred
scope: initial-hosted-pilot
architecture: docs/hosted-pilot-simple-server-architecture.md
---

# 초기 Hosted Pilot 실행 manifest

## 1. 목표 배치

```text
Browser -> lucy(Viewer + Baley API) -> Tailscale -> devhub(shared PostgreSQL)
```

- Lucy의 기존 English service 443/3001은 변경하지 않는다.
- devhub PostgreSQL은 loopback과 `tailscale0`에만 bind한다.
- PostgreSQL은 tailnet client에 role/password 방식으로 제공한다.
- PostgreSQL TLS와 SCRAM은 초기 운영 요구사항에서 제외한다.
- public/AWS private interface에 API나 PostgreSQL port를 열지 않는다.

아래 `HPS-*`는 manifest reference이며 Baley public Task ID가 아니다. 현재 세션에는
Baley write MCP가 없으므로 Task를 fixture나 DB 직접 변경으로 대신 등록하지 않는다.

## 2. 완료된 결정

### HPS-01 — 실제 host inventory

- devhub와 lucy의 OS, resource, listener, Docker, 기존 service와 private network를
  read-only로 확인했다.
- 결과: `docs/hosted-pilot-server-architecture.md`
- status: completed evidence

### HPS-02 — 초기 topology 결정

- Owner가 lucy를 Web/Application server로, devhub를 tailnet shared PostgreSQL
  server로 결정했다.
- 결과: `docs/hosted-pilot-simple-server-architecture.md`
- status: human decision complete

## 3. Wave 1 — repository 운영 baseline

### HPS-03 — production runtime contract

- backend와 Viewer의 versioned artifact를 생성한다.
- file-based DB/lease secret loading과 production startup validation을 구현한다.
- `/readyz`, build metadata, request ID, structured access log와 HTTP timeout을
  구현한다.
- predecessor: HPS-02
- human confirm: 불필요. test와 security review로 판정한다.
- status: completed evidence

### HPS-04 — Lucy Web/Application deployment template

- loopback `baley-server`, Viewer static artifact와 lightweight ingress template을
  구현한다.
- `/api/*` prefix 제거, SPA fallback, health/readiness와 rollback 경계를 포함한다.
- 기존 Lucy English listener와 container를 참조하거나 변경하지 않는다.
- predecessor: HPS-03
- human confirm: template 구현은 불필요. lucy 최초 설치 전에 필요하다.
- status: completed evidence

### HPS-05 — devhub shared PostgreSQL deployment template

- loopback과 명시적 Tailscale bind address만 사용하는 PostgreSQL template을
  구현한다.
- role/password, persistent volume, migration, logical backup과 restore runbook을
  포함한다.
- 다른 application과 database/role을 공유하지 않는 분리 예시를 제공한다.
- predecessor: HPS-03
- human confirm: template 구현은 불필요. devhub listener/DB 생성 전에 필요하다.
- status: completed evidence

### HPS-06 — deploy, backup과 rollback automation

- immutable release 배치, current symlink 전환, 이전 release rollback을 구현한다.
- DB migration 전 backup과 checksum, isolated restore verification 절차를 구현한다.
- predecessor: HPS-04, HPS-05
- human confirm: 코드 구현은 불필요. 운영 DB restore/교체는 매번 필요하다.
- status: completed evidence

HPS-03~06의 검증과 review 증거는
`docs/hosted-pilot-repository-wave-completion.md`에 정리한다.

## 4. Wave 2 — 실제 host staging

### HPS-07 — devhub PostgreSQL staging

- Tailscale listener, policy와 host firewall을 적용한다.
- shared service에 Baley database와 분리 role을 만들고 migration을 실행한다.
- 허용된 tailnet client 접속과 비허용/public 접속 거부를 검증한다.
- backup과 빈 DB restore drill을 수행한다.
- predecessor: HPS-05, HPS-06
- human confirm: 최초 server mutation 직전에 필요하다.

### HPS-08 — lucy Web/Application staging

- loopback API, Viewer와 private ingress를 설치한다.
- devhub DB 연결, service restart, readiness와 application rollback을 검증한다.
- Lucy English service가 영향받지 않았음을 확인한다.
- predecessor: HPS-04, HPS-07
- human confirm: 최초 server mutation 직전에 필요하다.

### HPS-09 — staging end-to-end acceptance

- login/logout, Workspace 전환, MCP, Task mutation, restart recovery, backup restore와
  rollback을 staging hostname에서 검증한다.
- predecessor: HPS-08
- human confirm: staging 수용 판정에 필요하다.

## 5. Wave 3 — 공개와 Pilot

### HPS-10 — production ingress와 hostname

- 기존 Lucy 443과 충돌하지 않는 별도 hostname/Cloudflare Tunnel을 연결한다.
- exact origin, Secure Cookie와 public health path를 검증한다.
- predecessor: HPS-09
- human confirm: DNS와 public exposure 직전에 필요하다.

### HPS-11 — 초기 서비스 운영 개시

- 첫 production Account, 초대 사용자, backup schedule과 incident channel을 연다.
- resource, DB connection, authentication failure와 backup age를 관측한다.
- predecessor: HPS-10
- human confirm: 실제 사용자 초대 직전에 필요하다.
- terminal intent: 초기 Hosted Pilot 운영 측정으로 이어지는 leaf다.

## 6. 현재 실행 범위

HPS-03~HPS-06의 repository 변경, test와 review는 완료했다. 실제 host mutation인
HPS-07 이후는 2026-08-02 Owner의 로컬 기능 마감 결정으로 보류한다. 로컬 Pilot DB는
원본으로 유지하며, 향후 재개할 때 구현 결과와 runbook을 다시 검토하고 새 승인을 받은
뒤 진행한다.
