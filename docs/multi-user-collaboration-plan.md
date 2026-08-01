---
type: implementation-plan
status: draft-for-owner-review
scope: hosted-multi-user
related:
  - docs/account-workspace-access-contract.md
  - docs/hosted-pilot-deployment-plan.md
  - docs/operations-quality-plan.md
---

# 다중 사용자 권한·동시성 기획

## 1. Pilot 범위

초기 Hosted Baley는 지인·친구가 사용하는 invite-only 서비스다. 일반 사용자의 기본
인증은 Sign in with Google이며, 공개 가입, Google 이외의 identity provider, Google API
권한, email 발송, Baley 자체 MFA와 조직 billing은 제외한다. 한 Account는 여러 Workspace에
참여할 수 있고, 각 Workspace의 membership과 역할은 서로 독립적이다.

## 2. 권한 주체 분리

현재 Workspace Owner와 service-wide Account 관리가 일부 섞여 있다. Hosted Pilot에서는
다음 세 경계를 분리한다.

| 주체 | 책임 | 할 수 없는 일 |
| --- | --- | --- |
| Site Operator | 서비스 bootstrap, Account 복구·비활성, incident 대응 | Workspace의 Task·Gate 결과를 임의 승인 |
| Workspace Owner | Workspace 설정, membership, Owner 이전, Agent token 관리 | 다른 Workspace와 global Account credential 관리 |
| Participant | 부여된 Viewer/Operator/Approver capability 수행 | membership·global Account 관리 |
| Agent | Workspace-scoped Operator command 수행 | 사람 approval/admin capability 행사 |

사람 전용 Task/Gate/Lane/Workspace 결정은 authenticated human session과 one-use approval
grant 경계를 유지한다. Site Operator도 Workspace의 제품 판단 권한을 자동으로 갖지 않는다.

## 3. Google identity, Account와 초대

Google은 사람의 인증만 제공하고 Baley authorization을 결정하지 않는다. 로그인 성공 뒤에도
Workspace membership과 capability를 Baley DB에서 매 요청 확인한다.

외부 identity는 다음 key로 저장한다.

```text
provider = google
issuer = https://accounts.google.com
subject = <verified ID token sub>
account_id = <Baley Account UUID>
```

Google `sub`는 Account 연결의 안정 식별자다. email은 표시와 invite 확인에 사용할 수 있지만
변경될 수 있으므로 primary key나 자동 account-link key로 사용하지 않는다. ID token의
signature, `aud`, `iss`, `exp`와 필요한 `email_verified`를 backend가 검증한다.

로그인에는 Google Identity Services의 redirect UX를 우선한다. GIS가 `credential`과
`g_csrf_token`을 Baley login endpoint로 POST하면 backend가 CSRF cookie/body 일치와 ID token을
검증한 뒤 기존 Baley Session/CSRF Cookie를 발급한다. Google ID token은 이 요청 이후
폐기하고 Baley Session token으로 사용하지 않는다. Google API 접근이 없으므로 access token과
refresh token은 요청·저장하지 않는다.

권장 초기 흐름:

1. Site Operator가 최초 Account와 Workspace Owner를 bootstrap한다.
2. Workspace Owner가 대상 Workspace와 역할을 지정해 one-time invite를 만든다.
3. raw invite token은 한 번만 표시하고 DB에는 hash만 저장한다.
4. invite는 만료 시간, 발급자, 대상 Workspace와 역할에 결속된다.
5. 사용자가 invite URL을 열면 backend가 raw token을 서버측 pending login transaction으로
   바꾸고 browser에는 opaque attempt ID만 남긴다.
6. 사용자는 Sign in with Google을 완료한다.
7. 신규 Google `sub`는 새 Baley Account와 external identity를 만들고 membership을 추가한다.
8. 기존에 연결된 Google `sub`는 기존 Account에 membership만 추가한다.
9. consume은 Account/external identity/membership 생성과 함께 한 transaction에서 한 번만
   성공한다.

초기에는 email을 보내지 않고 사용자가 안전한 외부 채널로 invite URL을 전달할 수 있다.
invite URL 자체를 credential로 취급하고 로그, referrer와 analytics에 남기지 않는다.

public account lookup은 제공하지 않는다. 동일 email의 local Account와 Google identity를
자동 연결하지 않는다. 기존 local Account에 Google identity를 연결하려면 로그인된 local
Session에서 별도 link flow와 재인증을 거친다. 기존 Account를 login ID로 바로 붙이는 현재
Owner 기능은 Pilot 전에 invite 흐름으로 대체하거나 Site Operator 전용으로 제한한다.

## 4. Account 복구와 관리

- 사용자는 로그인 상태에서 자신의 password를 변경하고 모든 Session을 revoke할 수 있다.
- Workspace Owner는 membership을 추가·비활성·역할 변경할 수 있지만, 사용자의 global
  password를 reset하거나 global Account를 disable하지 못한다.
- Google 사용자 복구는 Google Account 복구를 따른다. Baley는 Google credential을 reset하지
  않는다.
- local break-glass Account의 password 분실은 Site Operator가 offline recovery 절차로
  처리한다.
- recovery token은 hash-only, 짧은 expiry, single-use이며 모든 기존 Session을 revoke한다.
- Site Operator 행동은 별도 security Event에 남고 Workspace 제품 Event와 구분한다.
- 마지막 active Site Operator와 마지막 Workspace Owner 제거는 각각 거부한다.

현재 구현은 Account가 한 Workspace에만 속하면 Workspace Owner의 reset/disable을 허용한다.
이 동작은 로컬 단일 사용자에는 유용하지만 hosted tenant 경계에는 맞지 않으므로 변경한다.

## 5. 역할과 capability

사용자 화면은 단순하게 유지한다.

- `Owner`: Workspace 관리와 모든 human approval capability
- `Participant · Viewer`: 조회
- `Participant · Operator`: 일반 Task/Run/Record mutation
- `Participant · Approver`: 조회와 사람 승인

한 Participant에게 여러 역할을 조합하는 UI는 초기 범위가 아니다. 내부 capability bundle은
기존 `contracts/v1/capabilities.json`을 정본으로 유지한다. 역할 변경 즉시 새 요청부터
적용하며 active Session 자체를 재발급하지 않아도 membership fresh-read가 권한을 제한해야
한다.

Agent token은 다음 원칙을 유지한다.

- 하나의 Workspace와 하나의 Agent Actor에만 속한다.
- scope는 Operator bundle의 부분집합이다.
- raw token은 한 번만 표시하고 hash만 저장한다.
- 이름, 생성자, 생성일, 마지막 사용일, expiry와 revoke 상태를 사람이 조회할 수 있다.
- Task/Gate/Lane/Workspace의 사람 승인에는 human session이 발급한 one-use grant가 필요하다.

## 6. 동시성 모델

초기에는 현재의 Workspace-wide optimistic revision을 유지한다.

- 서로 다른 Workspace 변경은 독립적으로 진행된다.
- 같은 Workspace의 두 mutation이 같은 revision에서 시작하면 하나만 성공한다.
- stale mutation은 현재 revision과 재조회 필요 여부가 담긴 typed conflict를 반환한다.
- Viewer와 MCP는 자동으로 의미 변경을 재실행하지 않는다. fresh-read, preview 재생성 후
  사람이 보는 결과가 달라졌다면 다시 판단한다.
- 네트워크 응답 손실에 대비해 모든 생성·mutation은 idempotency identity를 가져야 한다.
- human-only command, 역할 변경, Owner 이전과 Account 복구는 blind retry하지 않는다.

global revision은 소수 사용자의 안전성을 우선한 선택이다. 서로 다른 Lane 작업이 자주
충돌한다는 Pilot evidence가 생길 때만 entity-level revision 또는 narrower lock을 검토한다.

## 7. 필수 동시성·격리 시나리오

1. 같은 Account가 Workspace A와 B에 참여하고 각각 다른 역할을 가짐
2. A를 보던 browser가 B로 전환할 때 A의 늦은 응답이 B state를 덮지 못함
3. A의 Session 또는 Agent token으로 권한 없는 B를 읽거나 쓰지 못함
4. 두 Owner가 같은 member 역할을 동시에 변경할 때 stale write가 명확히 실패함
5. 마지막 Owner 두 명을 동시에 비활성화해도 한 명은 남음
6. 같은 invite를 동시에 수락해도 Account/membership이 하나만 생김
7. invite 수락과 Owner revoke가 경쟁할 때 권한 없는 membership이 생기지 않음
8. approval grant 발급 뒤 revision 또는 Approver 역할이 바뀌면 execute가 실패함
9. Workspace 생성 응답 손실 뒤 exact retry는 동일 Workspace를 반환함
10. 한 Workspace 탈퇴·폐기가 global Account나 다른 Workspace membership을 바꾸지 않음

## 8. 보안·감사 기준

- 로그인 실패는 Account 존재 여부를 노출하지 않는다.
- password는 현재 Argon2id baseline을 유지하고 평문을 저장·기록하지 않는다.
- Session, CSRF, invite, recovery, Agent와 approval token 원문은 로그·Event·DB projection에
  나타나지 않는다.
- Google ID token, GIS CSRF token과 raw OAuth/GIS response도 저장하거나 로그하지 않는다.
- membership/role/Owner/invite/recovery/Account 상태 변경은 Actor provenance와 결과를
  security Event로 남긴다.
- tenant 경계에서 거부된 요청은 대상 Workspace의 audit stream을 오염시키지 않고 중앙
  security audit에 최소 정보만 남긴다.
- 사용자에게는 자기 Account의 active Session과 Agent token activity를 확인·폐기하는
  화면을 제공한다.

## 9. 구현 단위

1. Site Operator와 Workspace Owner의 Account authority 계약 분리
2. external identity와 pending login/invite transaction schema
3. Google ID token verifier, GIS CSRF 검증, HTTP endpoint와 Viewer button
4. invite/recovery token service와 기존/local Account link flow
5. Workspace Owner의 global reset/disable 제거와 Site Operator recovery command
6. active Session과 Agent token 목록·revoke UI
7. typed revision conflict와 Viewer/MCP 재조회 UX
8. 다중 Account·다중 Workspace PostgreSQL/HTTP/browser E2E
9. 권한·동시성 독립 보안 리뷰와 hosted staging 수용

## 10. 수용 기준

- 신규 사용자가 관리자에게 password를 전달하지 않고 invite와 Google 인증으로 가입한다.
- Google email 변경이 Account identity를 바꾸거나 중복 Account를 만들지 않는다.
- 위조·만료·잘못된 audience/issuer/CSRF의 Google credential은 Session을 만들지 못한다.
- Workspace Owner가 다른 사람의 global Account credential을 바꿀 수 없다.
- 한 Account가 여러 Workspace에서 서로 다른 권한을 안전하게 사용한다.
- Agent token은 사람 승인이나 Workspace administration을 행사하지 못한다.
- 동시 변경은 silent overwrite 없이 성공 또는 typed conflict로 끝난다.
- 마지막 Owner, invite single-use와 approval grant single-use가 경쟁 상황에서도 보장된다.
- 모든 권한 변경을 보안 Event에서 Actor와 결과 기준으로 재구성할 수 있다.

## 11. 참고 기준

- [OWASP Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)
- [OWASP Password Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)
- [Google Identity Services overview](https://developers.google.com/identity/gsi/web/guides/overview)
- [Google OpenID Connect reference](https://developers.google.com/identity/openid-connect/reference)
- [Verify Google ID tokens server-side](https://developers.google.com/identity/gsi/web/guides/verify-google-id-token)
