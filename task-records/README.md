# Baley Task Records

이 디렉터리는 상세계획, Handoff, 독립 Agent 리뷰, 리뷰 반영과 완료보고처럼 특정 Task의 실행 과정에만 필요한 기록을 보관한다.

- 원문과 version history는 이 repository와 Git이 보관한다.
- Baley Server에는 상대 경로, hash, 짧은 요약과 선택적 commit/blob metadata만 등록한다.
- 일반 repository 검색에서는 `.rgignore`의 `task-records/**` 규칙으로 제외한다.
- 작업할 때는 현재 Task에 연결된 정확한 파일 경로만 명시적으로 읽는다.
- 등록된 Record는 덮어쓰지 않고 `*-02.md`처럼 새 version과 새 `record_id`를 만든다.

## Baley 자체 개발의 bootstrap 예외

현재는 Baley Server가 없어 숫자 Task ID와 Run ID를 발급할 수 없다. 이 기간의 Record는 다음처럼 표시한다.

```yaml
task_id: null
run_id: null
registration_state: pending-bootstrap
```

이 파일은 아직 Baley에 등록된 불변 Record가 아니므로 실제 Task가 생성되면 숫자 `task_id`를 채우고 hash를 계산해 등록할 수 있다. 한 번 등록한 뒤에는 기존 파일을 수정하지 않는다. Bootstrap 예외는 Baley 제품의 별도 도메인 개념이 아니라 Baley Server를 만들기 위한 이 repository의 임시 운영 규칙이다.
