# Baley V1 contracts

이 디렉터리는 Baley V1에서 코드가 그대로 사용하는 이름과 값의 정본이다.

- `commands.json`: command 이름, 실행 경로, capability와 사람 승인 요구
- `states.json`: 상태값, 전이와 blocker/dependency 실행 규칙
- `diagnostics.json`: error, warning과 advisory code
- `capabilities.json`: capability와 role bundle

문서의 역할은 다음과 같이 분리한다.

1. `docs/baley-product.md`는 제품 의도와 UX 원칙을 설명한다.
2. `docs/baley-system-spec-v1.md`는 규범적 도메인 의미와 불변식을 정의한다.
3. 이 디렉터리는 command, 상태와 diagnostic의 정확한 literal을 고정한다.
4. HTTP API와 MCP는 위 계약을 구현하고 Skill은 Operator workflow를 설명한다.

문서가 이 파일의 목록을 복제하지 않도록 한다. 계약 변경은 관련 JSON, System Spec의 의미 설명과 인수 테스트를 같은 변경에서 갱신한다.
