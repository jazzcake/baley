# 2026-07-26 Baley DB 사고 및 재구성 보고

## 결과

독립 리뷰 중 운영 `baley` PostgreSQL DB가 integration test fixture로 잘못
사용되어 Workspace revision 194의 상태가 seed 상태로 대체됐다.

물리적인 완전 복구는 불가능했지만 repository Task Records, PM 승인 manifest,
completion reports와 확인된 fresh-read 메타데이터를 근거로 새 DB를
재구성했다. 재구성 DB는 최신 Baley server에서 검증한 뒤 운영 `baley` 이름으로
승격했다.

- 재구성 기준 revision: 194
- 복구 Record 반영 revision: 196
- Task #121 implementation report 후 revision: 197
- active Phase: `validate`
- Task #121: `implemented`, 사람 confirmation pending
- 손상 DB 보존 이름: `baley_damaged_20260726`

## 직접 원인

독립 리뷰 Agent가 다음 URL을 테스트 DB로 지정했다.

```text
postgres://baley:baley@127.0.0.1:54329/baley?sslmode=disable
```

`BALEY_TEST_DATABASE_URL`이라는 변수 이름만 신뢰했고 실제 database name이
`baley`인지 검증하지 않았다. integration suite는 disposable DB라는 전제에서
`TRUNCATE ... CASCADE` 후 `SeedDemo`를 실행하므로 운영 데이터가 제거됐다.

## 기여 요인

1. destructive integration helper에 database-name allowlist가 없었다.
2. Agent 위임 시 정확한 전용 DB URL과 금지 URL을 강제하지 않았다.
3. 운영 role과 test role이 동일했다.
4. `archive_mode=off`였고 base backup도 없어 PITR을 사용할 수 없었다.
5. legacy/current server가 여러 포트에 공존해 runtime 경계가 불명확했다.

## 물리 복구 시도

- Docker 종료 후 원본 VHDX 전체 복제 및 SHA-256 고정
- damaged DB custom-format dump와 volume tar 보존
- 복제 VHDX만 `ro,noload`로 연결
- ext4 journal, deleted inode, PostgreSQL relation file 복구 시도
- journal에 손상 전 directory 흔적은 일부 있었지만 relation file 집합은
  완전하게 복구되지 않음
- 불완전 파일은 운영 volume에 적용하지 않음

복구 이미지:

```text
D:\baley-recovery\2026-07-26\docker_data_pre_recovery.vhdx
SHA-256 CF65000ECAD4BECAD6B9379A5DEDD5E50B482147A4C630FBBCA1D05A98F3E0C4
```

재구성 dump:

```text
D:\baley-recovery\2026-07-26\baley-reconstructed-20260726.dump
SHA-256 777C1D993B34177EEC4B025E39E157D199FFB98DE6B231694039A388A7FA3739
```

## 논리 재구성 범위

- Workspace, Actor, Repository
- Build/Validate/Embedding Phases
- Server/Client/Art/Adoption Lanes
- Task #101, #104, #106, #110–#129
- Adoption manifest의 #117–#128 dependency topology
- Pilot Ready와 Embedding Gate 조건
- 31개 repository Task Record index
- 14개 terminal Run index
- 복구 자체를 나타내는 `recovery.reconstructed` Event

원래 Command/Event/approval attestation 전체 stream과 일부 Task의 원래 internal
UUID는 복원하지 않았다. 확인되지 않은 과거 audit payload를 발명하는 대신
재구성 Event에 이 경계를 명시했다. Task #112–#114의 상세 metadata는 Git history와
roadmap 근거로 부분 추론했다.

## 검증

- 최신 server가 재구성 graph를 정상 load
- Workspace revision/active Phase 확인
- Task #121 UUID, lane, Phase, status 확인
- 23 Tasks / 18 dependencies / 4 Gates 확인
- Gate status와 condition/automatic entry projection 확인
- 31 Records / 14 Runs 조회 확인
- 재구성 DB dump 생성 후 운영 이름으로 승격
- 손상 DB는 삭제하지 않고 `baley_damaged_20260726`로 보존
- Task #121 `task.report_implemented` 성공

## 재발 방지

integration suite에 다음 hard guard를 추가했다.

- database name에 독립된 `test` 또는 `testing` marker가 반드시 존재
- DB host는 loopback만 허용
- `baley`, `baley_reconstructed_20260726`, 원격 host는 test 시작 전에 거부

운영 URL을 넣은 회귀 실행이 `TRUNCATE` 전에 즉시 실패하고 Workspace revision
197이 유지되는 것을 확인했다.

추가 운영 조치:

1. 운영/test PostgreSQL role 분리와 운영 role의 destructive DDL 제한
2. test 전용 container/port 고정
3. 정기 `pg_dump`와 복구 연습
4. base backup + WAL archive/PITR 구성
5. 위임 Agent 요청에 exact test DB URL과 production denylist 포함
