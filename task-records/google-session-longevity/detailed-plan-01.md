---
baley_record: 1
record_id: "6101e2c5-6486-4312-bdea-a7d7b8c7050a"
task_id: 159
task_key: "google-session-longevity"
record_type: detailed-plan
run_id: "ee1fff75-8d19-4aa2-a961-01c77356557a"
created_at: "2026-09-01T00:00:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Google login session longevity plan

## 쉬운 설명

Baley가 Google 인증 뒤 자체 발급하는 브라우저 세션의 수명이 현재
30분 idle, 12시간 absolute로 고정되어 있다. 이 값을 운영에 맞는 장기 세션으로
바꾸고, 서버 재시작 뒤에도 지속되면서 명시적 보안 취소는 즉시 반영되게 한다.

## 왜 필요한가

Google 로그인 상태와 별개로 Baley 세션이 30분 동안 요청이 없으면 만료되기 때문에
사용자는 OAuth가 풀린 것처럼 반복 로그인하게 된다. OAuth 전환의 목적은 안전한
로그인과 장기 사용성의 결합이며, 짧은 하드코딩 수명은 그 목적에 맞지 않는다.

## 완료되면 무엇이 달라지는가

1. 세션 idle/absolute TTL을 명시적인 보안 정책으로 구성하고, 기본값과 허용 관계를
   서버에서 검증한다.
2. 운영 Compose는 최소 한 달의 실제 사용을 지원하는 정책을 명시한다.
3. Secure, HttpOnly, SameSite=Lax, server-side hash, CSRF, absolute expiry를 유지한다.
4. logout, account disable, membership 제거/역할 변경, Workspace archive, gateway revoke
   같은 기존 취소 경계는 장기 TTL보다 우선해 즉시 무효화된다.
5. 단위·통합 테스트, 컨테이너 재시작, 실제 provider/session endpoint로 배포를 검증한다.

## 범위·제외 사항

- 포함: 사람 브라우저 세션 정책, 환경 설정 검증, 쿠키 만료, idle touch, 운영 문서와
  Compose 배포 설정.
- 제외: Google access/refresh token 저장, MCP Agent credential 장기화, human-only 승인
  grant의 5분 수명 변경, 취소된 세션의 자동 복구.

## 구현 순서

1. 현재 30분/12시간 하드코딩과 DB touch·cookie expiry 동작을 회귀 테스트로 고정한다.
2. `SessionPolicy` 또는 동등한 명시적 설정을 추가하고 idle TTL이 absolute TTL보다
   길 수 없도록 시작 시 거부한다.
3. 운영 기본을 30일 idle/90일 absolute로 적용하고 Compose·운영 문서에 설정을 남긴다.
4. 세션 재시작 지속성 및 모든 명시적 revoke 경계를 테스트한다.
5. 전체 Go/frontend 검증, 프로덕션 빌드, Docker 배포와 실사용 endpoint 검증을 수행한다.

