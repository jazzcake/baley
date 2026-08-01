---
type: architecture-decision
status: accepted
scope: initial-hosted-pilot
decided_at: 2026-08-01
related:
  - docs/hosted-pilot-server-architecture.md
  - docs/hosted-pilot-deployment-plan.md
  - docs/account-workspace-access-operations.md
---

# 초기 Hosted Pilot 단순 서버 아키텍처

## 1. 결정

초기 Pilot은 두 서버를 Web/Application과 DB로만 나눈다.

```text
Browser
  |
  | HTTPS
  v
lucy — Web/Application server
  |- Baley Viewer static files
  |- lightweight ingress/reverse proxy
  `- baley-server API (127.0.0.1:8080)
             |
             | encrypted Tailscale network
             v
devhub — Database server
  `- shared PostgreSQL service on devhub Tailscale address
```

별도 API backend host/gateway, 다중 application instance와 Kubernetes는 초기
범위에서 제외한다. API가 없는 정적 Viewer만으로는 Baley가 동작하지 않으므로
`web server` 책임에는 Viewer와 Go API를 함께 포함한다.

## 2. `lucy` 책임

- versioned Viewer artifact 제공
- `/api/*`를 loopback `baley-server`로 reverse proxy
- `baley-server`를 dedicated service와 user로 실행
- login, Session, Workspace API와 MCP HTTP 경계 담당
- public TLS와 exact production origin 담당
- application log, health check와 rollback 담당

현재 Lucy English container가 host 443과 3001을 사용하고 있으므로 첫 배포에서
기존 listener와 container를 변경하지 않는다. Baley는 별도 hostname과 outbound
Cloudflare Tunnel 또는 충돌하지 않는 동등한 ingress를 사용한다. Cloudflare Tunnel은
두 서버를 분리하기 위한 계층이 아니라 기존 443 충돌을 피하는 최소 ingress다.

Baley 내부 경계는 다음과 같다.

```text
Cloudflare Tunnel
  -> Baley front gateway
       |- /api/* -> prefix 제거 -> 127.0.0.1:8080
       `- /*     -> Viewer SPA
```

`baley-server`는 현재 구현의 loopback-only bind를 유지한다. Viewer build는 다른
host/CI에서 수행하고 lucy에는 결과 artifact만 배치한다.

## 3. `devhub` 책임

- tailnet 내부에서 여러 용도로 사용할 shared PostgreSQL service
- Baley 전용 database와 application/migration/backup role
- 다른 application마다 분리된 database와 role
- PostgreSQL persistent volume과 backup staging
- migration 전 backup과 정기 logical backup
- 같은 tailnet에서 ACL/Grant가 허용된 client의 DB connection 제공
- DB health, disk, connection과 backup age 관측

PostgreSQL은 public interface나 AWS private interface에 publish하지 않는다.
loopback과 devhub의 `tailscale0` address에만 bind하고 다음을 함께 적용한다.

- Tailscale ACL/Grant: PostgreSQL port에 접근할 사용자, tag 또는 group을 명시
- host firewall: PostgreSQL port는 `tailscale0` inbound만 허용
- `pg_hba.conf`: tailnet source와 필요한 role/database 조합만 허용
- Tailscale이 전송 구간을 암호화하므로 초기에는 PostgreSQL TLS를 구성하지 않음
- 초기 PostgreSQL 인증은 role별 password 방식만 사용하고 SCRAM을 운영 요구사항으로 두지 않음
- `trust` 인증은 사용하지 않음

같은 tailnet에 있다는 사실은 network reachability만 제공한다. Database 인증을
대체하지 않으며, client마다 용도에 맞는 PostgreSQL role과 credential을 사용한다.
Baley application, migration/backup, 사람의 관리·개발 접속은 role을 분리한다.
다른 application은 Baley database나 role을 공유하지 않는다.

초기에는 tailnet 전체 member에게 무조건 port를 여는 대신 명시적인 Tailscale
group/tag를 권장한다. Owner가 tailnet 전체 member 접근을 원하면 Tailscale policy의
member group에 port를 허용할 수 있지만 PostgreSQL 인증과 `pg_hba.conf` 제한은
그대로 유지한다.

MongoDB와 Qdrant의 기존 all-interface bind는 Baley DB 설정에 복제하지 않는다.

## 4. public·private port

| host | listener | caller | 범위 |
| --- | --- | --- | --- |
| Cloudflare edge | 443 | Pilot browser | public |
| lucy Baley ingress | private/loopback | local cloudflared | host 내부 |
| lucy baley-server | 127.0.0.1:8080 | local ingress | loopback |
| devhub PostgreSQL | tailscale0:dedicated port | 허용된 tailnet client | tailnet-only |

Baley API 8080과 PostgreSQL port를 `0.0.0.0`에 publish하지 않는다.

## 5. 동일 origin

브라우저에는 Baley production origin 하나만 보인다.

```text
VITE_BALEY_API_URL=https://baley.<production-domain>/api
BALEY_VIEWER_ORIGINS=https://baley.<production-domain>
BALEY_ENV=production
BALEY_AUTH_MODE=enforced
BALEY_COOKIE_SECURE=true
BALEY_HTTP_ADDR=127.0.0.1:8080
```

front gateway는 `/api` prefix를 제거해 `/v1`, `/healthz`와 `/readyz`를 API에
전달한다. Session/CSRF cookie는 같은 public origin에서 유지한다.

## 6. 실제 자원 기준

### lucy

- 2 vCPU, 약 914 MiB RAM, 1 GiB swap
- 기존 Lucy English container 약 12 MiB
- Baley API, Viewer gateway와 cloudflared 합계 목표 256 MiB 이하
- build와 DB backup은 lucy에서 실행하지 않음
- devhub PostgreSQL 접속을 위해 lucy를 같은 tailnet에 등록

Go API와 정적 Viewer는 초기 소규모 Pilot에 적합하지만 memory 제한과 관측을
적용한다.

### devhub

- 2 vCPU, 약 1.9 GiB RAM, swap 없음
- 기존 MongoDB 약 397 MiB, Qdrant 약 22 MiB
- shared PostgreSQL은 384~512 MiB에서 시작하되 전체 database workload로 재조정
- production 전 최소 1 GiB swap 추가 또는 memory 증설 권장

기존 문서의 4 core·16 GB 가정은 사용하지 않는다.

## 7. secret과 배포 경계

다음 값은 Git, image, Compose literal, Task Record와 shell history에 넣지 않는다.

- PostgreSQL application/migration/backup credential
- `BALEY_LEASE_TOKEN_SECRET`
- Agent token
- Cloudflare Tunnel credential
- Tailscale node enrollment와 policy 관리 credential
- Google login production configuration
- backup encryption/storage credential

production 배포 전에 file-based secret loading, `/readyz`, build version, structured
log, backup/restore와 application rollback을 구현한다.

## 8. 초기 배포 순서

### Wave 1 — repository 준비

1. versioned Viewer와 `baley-server` artifact
2. lucy service/ingress template
3. devhub PostgreSQL production template
4. secret-file, `/readyz`, request log와 build metadata
5. migration/backup/restore/deploy/rollback scripts
6. Go, PostgreSQL, Viewer test와 독립 security review

### Wave 2 — devhub DB staging

1. `tailscale0` bind와 Tailscale ACL/Grant, host firewall 제한 확인
2. shared PostgreSQL service와 persistent volume 생성
3. Baley 전용 database와 application/migration/backup role 생성
4. migration과 bootstrap
5. lucy와 허용된 별도 tailnet client에서 role/password DB connection 확인
6. 허용되지 않은 tailnet client와 public/AWS interface에서 연결 거부 확인
7. logical backup과 빈 DB restore drill

### Wave 3 — lucy application staging

1. loopback API와 Viewer artifact 배치
2. local ingress에서 health/readiness 확인
3. staging hostname과 Cloudflare Tunnel 연결
4. login, Workspace 전환, logout과 MCP smoke test
5. Lucy English가 영향받지 않았는지 확인

### Wave 4 — Pilot 공개

1. production hostname과 exact origin 활성화
2. production Account와 invitation 검증
3. rollback point 확인 후 소규모 사용자 초대
4. memory, DB connection, disk와 backup age 관측

## 9. 사람 승인 경계

다음은 실제 변경 직전에 Owner 확인을 받는다.

- production hostname과 Cloudflare Tunnel 생성
- devhub PostgreSQL Tailscale listener와 Tailscale ACL/Grant 변경
- lucy의 tailnet enrollment
- lucy application service 설치와 시작
- production secret와 첫 Account bootstrap
- 운영 DB restore/교체
- production 공개와 Pilot 사용자 초대

repository 구현, 테스트와 독립 리뷰는 이 구조를 기준으로 먼저 완료할 수 있다.
