---
type: completion-evidence
status: completed
scope: HPS-03..HPS-06
architecture: docs/hosted-pilot-simple-server-architecture.md
completed_at: 2026-08-02
---

# Hosted Pilot repository 운영 baseline 완료

## 결과

실제 서버를 변경하기 전 필요한 repository wave를 완료했다.

- production runtime은 DB와 lease secret을 파일로 읽을 수 있고 production에서
  명시적 DB URL, HTTPS Viewer origin, secure cookie와 enforced auth를 요구한다.
- API는 migration version 16을 확인하는 `/readyz`, `/versionz`, request ID,
  최소 structured access log와 HTTP timeout을 제공한다.
- release builder는 Viewer, 정적 Linux amd64 server, migration, build metadata와
  SHA-256 manifest를 하나의 immutable release directory에 만든다.
- lucy template은 loopback API와 Caddy SPA/API gateway, systemd credential,
  one-shot migration, guarded symlink activation과 application rollback을 제공한다.
- devhub template은 PostgreSQL을 정확한 Tailscale IPv4에만 publish하고, 앱·migration·
  backup role을 분리하며 backup과 disposable restore verification을 제공한다.

## 검증

- 전체 Go test와 `go vet`: PASS
- PostgreSQL migration/integration 1~16과 aggregate acceptance: PASS
- frontend 14 files / 57 tests, typecheck와 production build: PASS
- release 생성, SHA-256 재검증, static Linux x86-64 ELF 확인: PASS
- 모든 shell script `bash -n`: PASS
- PowerShell release builder parser와 기존 output 보존 회귀: PASS
- Compose render와 Caddy validation: PASS
- `git diff --check`: PASS

Viewer production build에는 기존 약 1.9 MB JavaScript chunk warning이 남지만 초기 Pilot
기능을 막지는 않는다. devhub가 사용하는 legacy `docker-compose` 1.29 호환을 위해
Compose top-level `version`은 유지하며, 새 Compose에서는 obsolete warning만 발생한다.

## 자체 보안·운영 review에서 수정한 사항

- 기존 release output을 오류 정리 과정에서 삭제할 수 있던 경로를 제거했다.
- restore 검증은 이 실행이 생성한 고유한 disposable DB만 삭제하도록 제한했다.
- backup directory가 같은 timestamp의 기존 directory를 재사용하지 않게 했다.
- activation은 `/srv/baley/current`가 symlink가 아니면 중단하고, migration·restart·
  readiness 실패 시 이전 release 또는 정지 상태로 복원한다.
- cluster 전체 `pg_read_all_data` 대신 Baley schema에만 backup read 권한을 부여한다.
- PostgreSQL 초기 인증 option이 빈 data volume에만 적용된다는 경고와 설치 file mode를
  runbook에 명시했다.

## 남은 승인 경계

이 완료에는 `devhub` 또는 `lucy`의 서비스, listener, firewall, Tailscale policy,
secret, DNS와 공개 ingress 변경이 포함되지 않는다. 다음 단계 HPS-07은 devhub에 shared
PostgreSQL을 실제 설치하고 Baley database/role을 만드는 server mutation이므로 실행
직전에 Owner 확인이 필요하다.

현재 세션에는 Baley write MCP가 없으므로 HPS reference를 라이브 Task로 위장하거나
fixture/DB 직접 변경으로 등록하지 않았다.
