---
type: architecture
status: validated
authority: legacy_visual
last_active: 2026-07-17
when_to_read: "Phase 0 Visual MVP를 구현하거나 그래프의 데이터 흐름, 화면 경계와 기술 선택을 변경할 때"
affects:
  - docs/baley-product.md
  - docs/baley-roadmap.md
  - docs/baley-visual-grammar.md
---

# Baley Visual MVP 아키텍처

> 이 문서는 Phase 0에서 검증한 historical Visual fixture 아키텍처다. `done/running/blocked/ready`, Lane `close-out/discard`, `Phase.order`, Gate `reopened`와 `required/reference/unlocks`는 legacy 표현이며 서버 정본 모델은 `docs/baley-system-spec-v1.md`를 따른다.

## 1. 목적

이 문서는 Phase 0 Visual MVP를 구현하기 위한 프론트엔드 아키텍처를 정의한다. 현재 단계에서 검증할 대상은 데이터 저장이나 실제 task 실행이 아니라 다음 시각 언어다.

- 여러 lane의 독립적인 task DAG
- 전체 lane을 관통하는 Phase
- 현재 Phase에서 다음 Phase로의 전이 조건인 Gate
- Gate의 `required`, `reference`, `unlocks` 관계
- Gate 조건과 무관하지만 공통 Phase 안에 존재하는 task 경로
- Multi-lane, Lane Focus, Gate Focus 사이의 일관된 이동

서버 API, 데이터베이스, 인증, 권한, 그래프 편집, 실제 Git 및 AI 연동은 이 아키텍처의 범위가 아니다.

## 2. 핵심 결정

### 2.1 기술 구성

- 언어: TypeScript
- 애플리케이션: React
- 개발 및 빌드: Vite
- 그래프 렌더링과 viewport: React Flow
- 자동 layout: ELK.js layered algorithm
- 스타일: 전역 design token과 component별 CSS Module
- 시각 의미 규칙: `docs/baley-visual-grammar.md`
- 단위·컴포넌트 테스트: Vitest와 React Testing Library
- 핵심 사용자 흐름 검증: Playwright

React Flow는 node/edge 렌더링, 선택, 강조, 확대·축소와 이동만 담당한다. Baley의 도메인 규칙이나 view별 node 선택은 React Flow 객체에 넣지 않는다.

ELK.js는 lane과 Phase/Gate 경계를 고려한 계층형 자동 배치를 담당한다. 계산된 좌표는 렌더링 결과이며 fixture의 정본 데이터가 아니다.

별도 전역 상태 라이브러리는 도입하지 않는다. Phase 0의 상태는 선택, 현재 view와 viewport 정도이므로 React context와 reducer로 충분하다.

### 2.2 단일 fixture, 복수 projection

모든 화면은 하나의 정규화된 fixture를 정본으로 사용한다.

```text
fixture
  → validate
  → project(view, selection)
  → layout(projected graph)
  → render
```

- `validate`: ID 참조, dependency cycle, Phase 순서와 Gate 경계 등 fixture 오류를 개발 시점에 탐지한다.
- `project`: 같은 의미 모델에서 Multi-lane/Lane/Gate 화면에 필요한 node와 edge를 선택하고 축약한다.
- `layout`: projection 결과에 좌표와 크기를 부여한다.
- `render`: layout 결과를 React Flow node/edge로 변환한다.

각 화면용 fixture나 수작업 좌표를 따로 만들지 않는다.

### 2.3 도메인과 렌더러 모델 분리

도메인 ID, 관계와 상태는 React Flow의 `Node`와 `Edge`를 직접 정본으로 사용하지 않는다.

아래 TypeScript type은 Phase 0 Visual fixture 전용 legacy shape다. 서버 정본 모델과 Gate 관계는 `docs/baley-system-spec-v1.md`가 우선하며, 이후 구현에서 이 fixture를 migration한다.

```ts
type WorkspaceFixture = {
  phases: Phase[];
  lanes: Lane[];
  tasks: Task[];
  dependencies: Dependency[];
  gates: Gate[];
  gateLinks: GateLink[];
};

type Phase = {
  id: string;
  name: string;
  order: number;
};

type Lane = {
  id: string;
  name: string;
  goal: string;
  lifecycle: "active" | "close-out" | "discard";
  summary: string;
};

type Task = {
  id: string;
  laneId: string;
  phaseId: string;
  parentTaskId?: string;
  title: string;
  description: string;
  status: "done" | "running" | "blocked" | "ready";
  blocker?: string;
};

type Dependency = {
  id: string;
  fromTaskId: string;
  toTaskId: string;
};

type Gate = {
  id: string;
  name: string;
  fromPhaseId: string;
  toPhaseId: string;
  status: "open" | "ready" | "passed" | "reopened";
};

type GateLink = {
  gateId: string;
  taskId: string;
  kind: "required" | "reference" | "unlocks";
};
```

Visual MVP에서는 `fork` 계보를 fixture에 넣지 않으므로 Lane의 렌더링 lifecycle에서 제외한다. 제품 도메인에서 제거하는 것은 아니다.

Task의 `phaseId`는 현재 fixture에서 Phase 구간을 명시하기 위한 값이다. 향후 서버 모델의 저장 방식으로 확정하지 않는다.

## 3. 화면 구조

```text
AppShell
├─ TopBar
│  ├─ ViewSwitcher
│  └─ CurrentContext
├─ GraphStage
│  ├─ PhaseBands
│  ├─ LaneBands
│  ├─ TaskNodes
│  ├─ GateNodes
│  └─ TypedEdges
└─ InspectorPanel
```

### 3.1 Multi-lane View

- 세로축은 lane, 가로축은 Phase 진행 방향으로 사용한다.
- Phase는 모든 lane을 관통하는 배경 구간으로 표현한다.
- Gate는 `fromPhase`에서 `toPhase`로 향하는 전이 관계이며, 화면에서는 두 Phase 사이의 공통 경계에 배치한다.
- Gate 조건에 참여하지 않는 task도 자신의 Phase와 lane 안에 남긴다.
- close-out된 Research lane도 삭제하지 않고 종료 상태로 표시한다.

### 3.2 Lane Focus View

- 전체 task graph와 Phase의 가로 맥락을 유지한다.
- 선택한 lane의 task를 lane header와 같은 강도로 강조한다.
- 다른 lane의 task와 관계는 제거하거나 요약 node로 축약하지 않고 흐리게 표시한다.
- 선택 lane의 Gate 무관 경로는 숨기지 않는다.
- Phase 경계는 Multi-lane View와 같은 방향과 순서를 유지한다.

### 3.3 Gate Focus View

- 선택 Gate를 중심으로 `required`, `reference`, `unlocks` task를 구분한다.
- 각 task의 lane 소속을 유지한다.
- Gate 이전 Phase와 다음 Phase의 경계를 명시한다.
- 조건 충족 여부와 Gate 상태를 보여주되 통과 동작은 제공하지 않는다.

### 3.4 Task Inspector

선택한 task의 fixture에 존재하는 정보만 표시한다.

- 이름, 설명과 상태
- lane과 Phase
- blocker
- parent/child
- upstream/downstream dependency
- Gate 관계

GitBinding, AI execution과 event history는 제품상 필요한 영역임을 표시할 수 있지만, 가짜 데이터를 만들거나 동작하는 UI로 구현하지 않는다.

## 4. 그래프 projection

Projection은 도메인 모델을 화면별 `ProjectedGraph`로 변환하는 순수 함수다.

```ts
type ViewSpec =
  | { kind: "multi-lane" }
  | { kind: "lane"; laneId: string }
  | { kind: "gate"; gateId: string };

type ProjectedGraph = {
  nodes: ProjectedNode[];
  edges: ProjectedEdge[];
  phaseBands: ProjectedPhaseBand[];
  laneBands: ProjectedLaneBand[];
};
```

Projection 단계가 담당하는 것:

- 화면에 포함할 실제 node 선택
- Lane Focus의 전체 graph 보존과 비선택 lane dimming 정보 생성
- edge 의미 타입 보존
- 선택 node의 upstream/downstream 집합 계산
- 강조와 흐림 상태 계산

Layout이나 React component가 graph traversal을 직접 수행하지 않는다.

## 5. 자동 layout

### 5.1 방향과 제약

- 전체 진행 방향은 왼쪽에서 오른쪽이다.
- lane 순서는 fixture에서 안정적으로 유지한다.
- Phase 순서는 `Phase.order`를 따른다.
- Gate는 `fromPhaseId`와 `toPhaseId` 사이 경계에 둔다.
- task dependency는 같은 Phase 안에서도 진행 방향을 유지한다.
- parent/child는 blocking dependency와 다른 시각 타입을 사용한다.

### 5.2 두 단계 layout

1. ELK에 projection graph를 전달해 lane 내부 node와 edge 경로를 계산한다.
2. 계산 결과를 Phase band와 Gate 경계에 맞춰 정렬하고 전체 lane 높이를 확정한다.

Layout 입력을 직렬화한 key로 메모리 캐시한다. 선택 강조만 바뀔 때는 layout을 다시 계산하지 않는다. View, focus 대상 또는 graph 구조가 바뀔 때만 재계산한다.

초기 fixture 규모에서는 Web Worker를 사용하지 않는다. layout이 상호작용을 눈에 띄게 막는 것이 측정되면 별도 결정으로 도입한다.

## 6. 상태 관리와 URL

UI 상태:

```ts
type UiState = {
  view: ViewSpec;
  selectedNodeId?: string;
  inspectorOpen: boolean;
};
```

- fixture와 projection 결과는 UI 상태에 복제하지 않는다.
- 선택 node가 view 전환 후에도 존재하면 선택을 유지한다.
- 존재하지 않으면 해당 lane 또는 Gate context로 선택을 올린다.
- view는 URL로 표현한다: `/`, `/lanes/:laneId`, `/gates/:gateId`.
- 선택은 query parameter `?task=`로 표현해 화면 캡처와 검증 링크를 재현할 수 있게 한다.
- viewport 위치는 URL이나 저장소에 영속화하지 않는다.

## 7. 디렉터리 구조

```text
src/
├─ app/
│  ├─ App.tsx
│  ├─ routes.tsx
│  └─ ui-state.tsx
├─ domain/
│  ├─ model.ts
│  ├─ selectors.ts
│  └─ validate-fixture.ts
├─ fixtures/
│  └─ pilot-ready.ts
├─ graph/
│  ├─ projection/
│  │  ├─ multi-lane.ts
│  │  ├─ lane-focus.ts
│  │  ├─ gate-focus.ts
│  │  └─ highlight.ts
│  ├─ layout/
│  │  ├─ elk-layout.ts
│  │  └─ layout-types.ts
│  └─ adapters/
│     └─ react-flow.ts
├─ components/
│  ├─ graph-stage/
│  ├─ nodes/
│  ├─ edges/
│  ├─ inspector/
│  └─ navigation/
├─ styles/
│  ├─ tokens.css
│  └─ global.css
└─ test/
   └─ setup.ts
```

의존 방향은 항상 다음을 따른다.

```text
components → graph projection/layout → domain
app        → components, graph, fixture
domain     → 외부 UI 라이브러리에 의존하지 않음
fixture    → domain type에만 의존
```

## 8. 대표 fixture

두 Phase와 하나의 전환 Gate로 시작한다.

```text
Phase: Build

Server lane
  API 설계 → API 구현 ─────────┐

Client lane                    │
  화면 설계 → Pilot UI ────────┼→ Pilot Ready Gate
       └→ 접근성 개선           │

Art lane                       │
  Asset 제작 ──────────────────┘

Research lane
  조사 → 실험 → 결과 정리 → close-out
  ※ Gate 조건에는 참여하지 않지만 Build Phase에는 포함

Phase: Validate

Pilot Ready Gate → 사용자 테스트
```

`Pilot Ready`의 `required`는 API 구현, Pilot UI와 Asset 제작이다. 사용자 테스트는 `unlocks`다. 접근성 개선과 Research lane task는 Gate link가 없다.

## 9. 검증 전략

### 9.1 정적 fixture 검증

- 모든 ID가 유일하다.
- 참조 ID가 존재한다.
- dependency에 cycle이 없다.
- task는 존재하는 lane과 Phase에 속한다.
- Gate의 이전·다음 Phase가 존재하고 순서가 증가한다.
- `required`와 `reference` task는 Gate 이전 Phase에 있다.
- `unlocks` task는 Gate 다음 Phase에 있다.

### 9.2 단위 테스트

- 세 view projection의 node/edge 집합
- upstream/downstream 강조 범위
- Gate와 무관한 경로 보존
- Lane Focus의 전체 graph 보존과 비선택 lane dimming
- 잘못된 fixture 거부
- layout 결과의 유한 좌표, 겹침과 안정된 순서

픽셀 단위 좌표 전체를 snapshot으로 고정하지 않는다. 관계, 순서와 겹침 여부를 검증한다.

### 9.3 브라우저 검증

- 세 view 간 이동
- task 선택과 Inspector 표시
- upstream/downstream 강조
- Gate 조건 펼치기
- zoom, pan과 fit view
- URL 직접 진입과 새로고침
- 주요 viewport 크기에서 핵심 node와 label 가독성

## 10. 구현 순서

1. Vite/React/TypeScript 기반과 테스트 환경 구성
2. domain type, 대표 fixture와 validator 작성
3. Multi-lane projection 및 ELK layout 연결
4. Phase/lane band, task/Gate node와 typed edge 렌더링
5. 선택, 강조와 Task Inspector
6. Lane Focus와 Gate Focus projection
7. URL navigation, zoom/pan과 view 전환 마감
8. Gate V0 체크리스트 기반 브라우저 검증과 화면 기록

## 11. Phase 0에서 고정하지 않는 것

- 서버 entity와 DB schema
- HTTP API payload
- Task 상태의 최종 state machine
- Phase와 task 관계의 영속 저장 방식
- Gate 승인 및 reopen 정책
- 대규모 graph virtualization
- Lane Group
- GitBinding, Execution과 Event의 상세 schema

Visual MVP에서 만들어지는 TypeScript type은 시각 검증용 계약이며 향후 Go 도메인 모델의 선행 schema가 아니다.

## 12. 완료 조건

아키텍처 구현 완료는 기능 개수보다 Gate V0 판정 가능 여부로 결정한다.

- 하나의 fixture에서 세 view가 파생된다.
- Phase가 전체 lane을 관통하고 Gate가 Phase 경계로 읽힌다.
- dependency와 Gate link가 시각적으로 혼동되지 않는다.
- Gate 조건과 무관한 task와 lane이 자연스럽게 남는다.
- 자동 layout 외에 수동 node 좌표가 fixture에 없다.
- Inspector와 강조로 관계를 추적할 수 있다.
- 새로고침 가능한 URL로 각 검증 장면을 재현할 수 있다.
