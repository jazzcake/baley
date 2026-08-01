---
type: architecture-decision-proposal
status: superseded
scope: hosted-pilot
observed_at: 2026-08-01
superseded_by: docs/hosted-pilot-simple-server-architecture.md
related:
  - docs/hosted-pilot-deployment-plan.md
  - docs/hosted-pilot-execution-manifest.md
  - docs/account-workspace-access-operations.md
---

# Hosted Pilot 서비스 서버 아키텍처

> 이 제안은 2026-08-01 Owner 결정으로 단순화되었다. 초기 Pilot의 정본은
> `docs/hosted-pilot-simple-server-architecture.md`다. 이 문서는 향후 front/backend
> 분리 확장안과 read-only inventory 근거로만 유지한다.

## 1. 결론

Baley Hosted Pilot은 기존 서비스를 건드리지 않는 다음 구조로 배치한다.

```text
Browser
  |
  | HTTPS: baley.<production-domain>
  v
Cloudflare edge
  |
  | outbound Cloudflare Tunnel
  v
lucy
  |- 기존 Lucy English container (현재 443/3001, 변경하지 않음)
  |- Baley front gateway (Caddy, private listener)
  |    |- /          -> versioned Viewer static artifact
  |    `- /api/*     -> prefix 제거 후 Tailscale upstream
  `- cloudflared (Baley 전용 tunnel)
                         |
                         | encrypted Tailscale link
                         v
devhub
  |- Tailscale Serve/private gateway
  |    `- 127.0.0.1:8080 -> baley-server
  |- baley-server (systemd, dedicated user)
  `- PostgreSQL (dedicated container and volume, loopback only)
```

Cloudflare Tunnel은 기존 Lucy container가 점유한 443을 변경하지 않고 별도 Baley
hostname을 공개한다. Tailscale은 front와 backend 사이를 암호화하며, API와
PostgreSQL은 public interface에 직접 노출하지 않는다.

## 2. 2026-08-01 read-only inventory

공개 IP, credential, PEM 경로와 secret 값은 기록하지 않는다.

### `devhub` — backend

| 항목 | 실제 확인 결과 |
| --- | --- |
| OS | Ubuntu 24.04 LTS, x86_64 |
| 자원 | 2 vCPU, 약 1.9 GiB RAM, swap 없음 |
| disk | 약 58 GiB 중 12 GiB 사용, 46 GiB 여유 |
| container runtime | Docker 29.1.3, legacy `docker-compose` 1.29.2 |
| 기존 container | MongoDB 7, Qdrant |
| 기존 memory | MongoDB 약 397 MiB, Qdrant 약 22 MiB |
| network | Lucy와 같은 AWS private subnet, Tailscale 설치·실행 중 |
| host firewall | UFW inactive |
| listeners | SSH, MongoDB 27017, Qdrant 6333/6334가 all-interface bind |
| web gateway | Nginx, Caddy, Apache 없음 |

기존 문서의 4 core·16 GB 전제는 실제 서버와 다르다. 이 문서의 확인값을 Hosted
Pilot 자원 계획의 기준으로 사용한다.

MongoDB와 Qdrant의 all-interface bind는 Baley 배포와 별개인 기존 위험이다. AWS
Security Group이 외부 접근을 차단하는지 확인하기 전에는 host bind만 보고 public
접근 가능 여부를 단정하지 않는다. Baley port에는 이 패턴을 반복하지 않는다.

### `lucy` — frontend

| 항목 | 실제 확인 결과 |
| --- | --- |
| OS | Ubuntu 22.04 LTS, x86_64 |
| 자원 | 2 vCPU, 약 914 MiB RAM, 1 GiB swap |
| disk | 약 39 GiB 중 7.2 GiB 사용, 32 GiB 여유 |
| container runtime | Docker 28.2.2, standalone Compose 5.1.1 |
| 기존 container | Lucy English 1개, 약 12 MiB 사용 |
| public listeners | 443, 3001과 SSH |
| TLS | Cloudflare Origin wildcard certificate 사용 중 |
| network | devhub와 같은 AWS private subnet, 상호 ping·private SSH port 도달 가능 |
| host firewall | UFW inactive |
| web gateway | Nginx, Caddy, Apache 없음 |

Lucy English는 443과 3001을 Docker로 직접 publish하며 restart policy가 없다. 첫
Baley 배포에서는 이 container, certificate mount, listener와 restart policy를
변경하지 않는다.

## 3. 배치 결정

### Frontend: `lucy`

- Viewer는 CI 또는 개발 host에서 build한 immutable static artifact만 배치한다.
- Baley 전용 Caddy는 정적 파일과 SPA fallback을 제공한다.
- `handle_path /api/*`로 `/api` prefix를 제거하고 backend에 reverse proxy한다.
- cloudflared는 Baley 전용 tunnel과 hostname만 관리한다.
- Caddy와 cloudflared는 기존 Lucy Compose project와 분리한 service, network,
  directory와 restart policy를 사용한다.
- 기존 host 443에는 bind하지 않는다. Cloudflare Tunnel은 outbound connection만
  사용하고 Baley front gateway는 public host port를 열지 않는다.

### Backend: `devhub`

- `baley-server`는 dedicated Linux user와 hardened systemd unit으로 실행한다.
- 현재 application의 loopback-only bind를 유지해 `127.0.0.1:8080`에서만 듣는다.
- Tailscale Serve 또는 동등한 tailnet-only gateway가 loopback API를 lucy에만
  제공한다. Tailscale Funnel은 사용하지 않는다.
- PostgreSQL은 Baley 전용 container, network, role, volume을 사용한다.
- systemd host process가 접근할 수 있도록 DB port는 필요할 때만
  `127.0.0.1:<dedicated-port>`에 publish하며 public interface에는 bind하지 않는다.
- migration은 application 시작과 분리된 one-shot command로 실행한다.

### 동일 origin

브라우저에는 `https://baley.<production-domain>` 하나만 노출한다.

- Viewer build: `VITE_BALEY_API_URL=https://baley.<production-domain>/api`
- API runtime: `BALEY_VIEWER_ORIGINS=https://baley.<production-domain>`
- browser는 `/api/v1/...`를 호출한다.
- Caddy는 `/api`를 제거해 backend의 `/v1/...`로 전달한다.
- Secure Session/CSRF cookie는 같은 public origin에서 유지한다.

## 4. port와 trust boundary

| 위치 | listener | 접근 주체 | 공개 여부 |
| --- | --- | --- | --- |
| Cloudflare edge | 443 | Pilot browser | public |
| lucy Baley gateway | private container/loopback | local cloudflared | private |
| devhub Tailscale gateway | tailnet HTTPS | lucy node만 | private encrypted |
| devhub baley-server | 127.0.0.1:8080 | local Tailscale gateway | loopback |
| devhub PostgreSQL | loopback/container network | baley-server, migration, backup | private |

AWS Security Group은 최소한 다음 원칙을 만족해야 한다.

- Baley용 8080과 PostgreSQL port를 inbound에 추가하지 않는다.
- lucy의 기존 443/3001 규칙은 이번 작업에서 바꾸지 않는다.
- SSH는 현재 운영 경계를 유지하되 가능한 경우 Tailscale 또는 제한된 source만 허용한다.
- devhub의 기존 MongoDB/Qdrant 규칙은 별도 보안 점검 대상으로 기록한다.

## 5. 자원 예산

### devhub

2 GiB RAM에서 기존 MongoDB와 Qdrant를 함께 운영하므로 초기 상한을 둔다.

- PostgreSQL: 384~512 MiB 범위에서 시작
- baley-server: systemd `MemoryMax` 256 MiB 기준
- migration/backup: 동시 실행을 피하고 일시 사용량을 관측
- 최소 1 GiB swap 추가 또는 instance memory 증설 중 하나를 production 전 수행

swap이 없는 현재 상태에서도 작은 staging은 가능하지만, OOM 상황에서 DB나 기존
서비스가 종료될 수 있으므로 production 기준으로 승인하지 않는다.

### lucy

- Viewer static gateway와 cloudflared 합계 목표: 128 MiB 이하
- build는 lucy에서 수행하지 않는다.
- Lucy English의 사용량과 swap pressure를 함께 관측한다.

## 6. secret과 데이터

Git, image, Compose literal과 Task Record에 secret을 넣지 않는다.

- PostgreSQL application/migration/backup credential
- `BALEY_LEASE_TOKEN_SECRET`
- Agent token
- Cloudflare Tunnel credential
- Tailscale node enrollment credential
- Google login production configuration
- off-host backup encryption/storage credential

production 배포 전 Baley가 mounted file 또는 systemd credential에서 DB와 lease
secret을 읽는 기능을 구현한다. credential 원문을 shell history나 process argument에
넣지 않는다.

PostgreSQL은 매일 custom-format logical backup을 만들고 checksum, age와 exit
status를 기록한다. backup은 암호화해 다른 host/object storage로 옮기며, 월 1회
빈 DB에 실제 restore를 검증한다.

## 7. 배포 전 제품 gap

현재 repository 상태로 production server에 바로 배포하지 않는다. 다음 구현이
먼저 필요하다.

1. versioned backend binary/Viewer artifact와 build metadata
2. production 전용 deployment template와 systemd/Compose unit
3. secret-file 또는 systemd credential loading
4. DB·migration 상태를 포함하는 `/readyz`
5. HTTP timeout, structured access log, request ID와 credential redaction
6. backup/restore/deploy/rollback script와 runbook
7. production login 방식과 Google identity/invite 경계

## 8. 단계별 적용

### Wave 0 — 결정과 안전 기반

1. 이 architecture와 production hostname을 Owner가 확정한다.
2. lucy를 tailnet에 등록하고 devhub와 ACL을 Baley traffic으로 제한한다.
3. devhub에 swap을 추가하거나 memory를 증설한다.
4. AWS Security Group의 실제 inbound를 read-only 확인한다.

### Wave 1 — repository 구현

1. production artifact와 non-secret template 구현
2. secret-file, `/readyz`, build/version, logging 구현
3. backup/restore/deploy/rollback 자동화
4. 전체 Go, PostgreSQL, Viewer, security review 통과

### Wave 2 — backend staging

1. devhub에 isolated PostgreSQL과 baley-server 배치
2. migration, bootstrap, loopback health/readiness 확인
3. Tailscale private endpoint에서만 API smoke test
4. backup과 빈 DB restore drill

### Wave 3 — frontend staging

1. lucy에 isolated Viewer gateway와 cloudflared 배치
2. staging hostname에서 login, Workspace 전환, logout 검증
3. Lucy English 443/3001과 container가 영향받지 않았음을 확인

### Wave 4 — Pilot cutover

1. production hostname/DNS 활성화
2. production Account와 invitation flow 검증
3. rollback 가능한 상태에서 소규모 사용자 초대
4. disk, memory, readiness, backup age와 인증 실패율 관측

## 9. 사람 승인 경계

다음은 구현 Agent가 임의로 실행하지 않는다.

- 이 topology와 production hostname 확정
- lucy Tailscale enrollment와 Cloudflare Tunnel/DNS 생성
- AWS Security Group 변경
- devhub swap/instance 변경
- production secret와 Account bootstrap
- 운영 DB restore 또는 교체
- production hostname 공개와 Pilot 사용자 초대

코드, template, test, security review와 staging artifact 생성은 이 결정 이후 독립적으로
완료할 수 있다.

## 10. 현재 권장 Owner 선택

- public hostname: 기존 Cloudflare zone의 별도 `baley` subdomain
- ingress: 기존 Lucy 443을 유지하는 Cloudflare Tunnel
- private link: lucy에도 Tailscale을 설치하고 devhub와 tailnet-only 연결
- backend runtime: host systemd `baley-server` + loopback PostgreSQL container
- capacity: devhub에 최소 1 GiB swap을 추가하고 staging 측정 후 instance 증설 판단
- backup: encrypted off-host object storage, daily 7개 + weekly 4개 보존

이 선택은 기존 두 서비스의 listener와 container를 변경하지 않으면서 현재 서버
자원에서 가장 작은 운영 단위를 만든다.
