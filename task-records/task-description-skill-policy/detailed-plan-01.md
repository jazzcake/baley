---
baley_record: 1
record_id: "f0e89d6b-4536-45eb-bcca-68dbf76b1443"
task_id: 160
task_key: "task-description-skill-policy"
record_type: detailed-plan
run_id: "a5f9991e-7a9a-4d59-9c70-12eb091aeaf4"
created_at: "2026-09-01T00:00:00+09:00"
created_by: "codex"
registration_state: pending
supersedes: null
---

# Baley Task description Skill policy plan

## 쉬운 설명

Baley Agent가 Task를 생성하거나 내용을 바꿀 때 `currentSummary`와 사람이 읽기 쉬운
네 부분 설명을 항상 작성하도록 원본 Skill 규칙을 바꾸고, 공식 설치 스크립트로 새
플러그인 캐시 버전을 배포한다.

## 왜 필요한가

현재 Skill은 `currentSummary`를 선택 사항으로 두어 Task마다 설명 품질이 달라진다.
설치 캐시를 직접 고치면 다음 업데이트에서 사라지므로 repository의 canonical Skill을
원본으로 삼고 기존 패키징·cachebuster·재설치 흐름을 사용해야 한다.

## 완료되면 무엇이 달라지는가

1. 모든 `task.create`와 Task 내용 변경에 짧고 쉬운 `currentSummary`가 반드시 포함된다.
2. `description`은 기본적으로 `쉬운 설명`, `왜 필요한가`, `완료되면 무엇이 달라지는가`,
   `범위·제외 사항` 순서를 따른다.
3. 기존 Task를 최신화할 때도 같은 형식을 자동 적용한다.
4. repository 원본 Skill, 로컬 personal marketplace source, 설치 cache가 같은 새 버전을
   가리키며 새 Codex 세션에서 규칙을 로드할 수 있다.

## 범위·제외 사항

- 포함: `.agents/skills/baley-manage-work`, Skill 검증, repository plugin packaging,
  cachebuster, personal marketplace 재설치, 설치 결과 비교.
- 제외: 이번 단계에서 MCP/server가 `currentSummary` 누락 요청을 거부하는 계약 강제,
  기존 모든 Task의 일괄 데이터 마이그레이션, 설치 cache 직접 편집.

## 구현 순서

1. canonical Skill의 Task summary 지침을 명령형 필수 규칙으로 교체한다.
2. 생성·수정·최신화와 네 부분 본문 형식의 적용 범위를 명확히 하고 예외를 최소화한다.
3. `quick_validate.py`로 두 Baley Skill을 검증하고 plugin manifest validator를 실행한다.
4. repository installer로 원본을 personal plugin에 복사하고 cachebuster 후 재설치한다.
5. 원본·plugin source·설치 cache의 Skill hash와 활성 버전을 비교해 배포를 검증한다.

