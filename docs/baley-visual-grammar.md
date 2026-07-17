---
type: design-spec
status: active
authority: legacy_visual
last_active: 2026-07-17
when_to_read: "Baley의 그래프 색상, 형태, 선, 상태와 focus 표현을 구현하거나 변경할 때"
affects:
  - docs/baley-product.md
  - docs/baley-visual-mvp-architecture.md
---

# Baley 그래프 Visual Grammar

> 이 문서의 Task `done/running/blocked/ready`, Lane `close-out/discard`, `Phase.order`, Gate `reopened`와 `required/reference/unlocks` edge는 Phase 0 Visual MVP를 기록한 legacy 표현이다. 서버 기반 Viewer migration의 정본 상태와 관계는 `docs/baley-system-spec-v1.md`를 따른다.

## 1. 목적

Baley의 그래프에서 색상, 형태, 공간, 선과 명암이 서로 다른 의미를 일관되게 전달하도록 한다. 한 시각 채널에 여러 의미를 겹치지 않는 것이 핵심 원칙이다.

```text
색상       → Lane 소속
공간과 경계 → Phase
형태       → Task와 Gate의 객체 종류
아이콘·badge → Task 상태와 Lane 생명주기
선 스타일   → 관계 종류
명암과 굵기 → 선택, focus와 강조
```

색상만으로 상태나 관계를 전달하지 않는다.

## 2. 참고 기준

- [GitHub Primer color system](https://primer.style/foundations/color): 기능적 색상과 대비 원칙
- [Atlassian color foundations](https://atlassian.design/foundations/color): 의미 기반 color role과 접근성
- [IBM Carbon data visualization color](https://carbondesignsystem.com/data-visualization/color-palettes/): 범주형 데이터의 구별 가능한 색상 운용
- [React Flow custom nodes](https://reactflow.dev/learn/customization/custom-nodes): 그래프 node의 의미별 표현 방식

이 제품들은 시각 규칙의 참고일 뿐 Baley의 도메인 구조를 가져오지 않는다.

밝은 정보 계층은 monday.com의 작업 화면을 추가 참고한다. 테이블, group, column 구조는 차용하지 않고 다음 특성만 Baley의 그래프 문법에 적용한다.

- 밝은 중성 canvas와 흰색 surface
- 작은 면적에 사용하는 선명한 기능 색상
- 얇고 차가운 경계선
- 충분한 여백과 높은 텍스트 대비
- 읽을 수 있는 수준을 유지하는 focus dimming

## 3. Lane

Lane은 그래프의 기본 회상 단위이므로 색상의 주 소유자다.

Lane 색상은 다음 위치에만 반복한다.

- Lane header의 왼쪽 rail
- Lane 안 Task node의 상단 3px accent
- Lane Focus의 context marker
- 축약된 외부 Lane 표시

Task 배경 전체나 dependency edge 전체를 Lane 색으로 채우지 않는다. 큰 면적에 색을 쓰면 상태와 focus 표현을 방해한다.

초기 categorical palette:

| 순서 | 이름 | 값 |
|---|---|---|
| 1 | Teal | `#00A887` |
| 2 | Blue | `#579BFC` |
| 3 | Amber | `#FDAB3D` |
| 4 | Purple | `#A25DDC` |
| 5 | Green | `#00C875` |
| 6 | Coral | `#E8697D` |

색은 Lane ID에 결정적으로 배정해 view 전환 후에도 바뀌지 않게 한다. 6개를 초과하면 명도 차이를 추가하기보다 palette를 확장하거나 Lane 축약을 우선한다.

## 4. Phase

Phase는 색상 범주가 아니라 전체 Lane을 관통하는 공간 구간이다.

표현 규칙:

- 그래프 상단에 `PHASE 번호 + 이름`을 표시한다.
- Phase 사이에 세로 경계와 충분한 간격을 둔다.
- Gate는 Phase 경계 위에 배치한다.
- 현재 Phase 이름과 경계는 진하게, 이전·다음 Phase는 낮은 명암으로 표시한다.
- 배경색은 사용하지 않거나 매우 약한 중성 명암만 사용한다.
- Phase마다 서로 다른 hue를 사용하지 않는다.

확대·축소와 이동 시 Phase header와 경계는 그래프 좌표계를 함께 따른다.

## 5. Task

Task는 기본적으로 동일한 직사각형 card 형태를 사용한다.

- 배경: 중성 surface
- 테두리: 중성 1px
- 상단 accent: Lane 색상 3px
- 제목: 가장 강한 텍스트
- Lane 이름: 작은 보조 텍스트
- 상태: 아이콘과 짧은 label

Task 상태는 Lane 색을 덮어쓰지 않는다.

| 상태 | 아이콘/형태 | 추가 표현 |
|---|---|---|
| `done` | check | 제목 또는 상태 label의 낮은 명암 |
| `running` | play 또는 pulse dot | 얇은 activity indicator |
| `blocked` | lock 또는 warning | blocker badge와 선택적 사선 pattern |
| `ready` | hollow circle | 별도 강조 없음 |

`blocked`도 빨간 테두리 전체로 표현하지 않는다. 경고 색은 badge와 아이콘에 제한한다.

## 6. Gate

Gate는 Task와 다른 객체이며 Phase 전환점이다.

- Task와 다른 compact hexagon 또는 diamond-accent card 형태를 사용한다.
- Phase 경계를 가로지르는 위치에 둔다.
- 흰색 surface와 선명한 blue-violet accent outline을 사용해 Task와 구별한다.
- 이름, 상태와 `완료 required / 전체 required`를 표시한다.
- Gate Focus 진입 affordance를 제공한다.

Gate 상태:

| 상태 | 표현 |
|---|---|
| `open` | hollow progress ring |
| `ready` | solid progress ring + 승인 대기 label |
| `passed` | check seal + 통과 시각 |
| `reopened` | 향후 후보. V1에서는 Gate reopen과 Phase rollback을 지원하지 않음 |

## 7. 관계선

관계의 의미는 색보다 형태와 방향으로 구분한다.

| 관계 | 선 문법 |
|---|---|
| Task dependency | 중성 실선, 단일 화살표 |
| Parent/child | 얇은 중성 bracket 또는 비차단 실선 |
| Gate `required` | 굵은 실선, Task → Gate |
| Gate `reference` | 가는 점선, Task → Gate |
| Gate `unlocks` | 이중 짧은 dash 또는 강조 화살표, Gate → Task |

교차가 불가피하면 Gate 관계가 dependency보다 위에 보이게 한다. Edge label은 기본적으로 숨기고 선택 또는 Gate Focus에서만 표시한다.

## 8. 선택과 Focus

선택은 새로운 의미 색상을 추가하지 않는다.

- 선택 node: 2px blue focus ring과 약한 elevation
- upstream/downstream: 선 굵기와 불투명도 증가
- 직접 연결되지 않은 node와 edge: 불투명도 감소
- focus 대상 Lane 또는 Gate: 구조는 유지하고 주변만 축약
- keyboard focus: 선택과 구분되는 고대비 outline

흐려진 node도 이름과 상태를 읽을 수 있게 최소 55% 수준의 가시성을 유지한다.

## 9. Lane 생명주기

Lane 색은 생명주기와 무관하게 유지한다. 생명주기는 Lane header에서 badge와 icon으로 표시한다.

| 생명주기 | 표현 |
|---|---|
| `active` | 기본 header |
| `close-out` | check badge, 전체 명암 소폭 감소 |
| `discard` | stop badge, 이유 확인 affordance |
| `fork` 계보 | 분기 icon과 원본 Lane 연결 |

Close-out과 discard를 회색 처리해 삭제된 것처럼 보이게 하지 않는다.

## 10. 밀도와 Label

- 기본 zoom에서 Task 제목, 상태와 Lane 소속을 읽을 수 있어야 한다.
- 축소 시 설명부터 숨기고 제목과 상태 icon을 마지막까지 유지한다.
- Edge label은 상시 노출하지 않는다.
- 약어는 Phase/Gate 이름에서 사용하지 않는다.
- 영문 상태 label과 한글 설명을 한 화면에서 혼용할 때 casing 규칙을 통일한다.

## 11. 접근성

- 색상만으로 Lane, 상태 또는 관계를 전달하지 않는다.
- 작은 텍스트도 배경 대비를 확보한다.
- 모든 선택 가능한 node는 keyboard focus가 가능해야 한다.
- 상태 icon에는 접근 가능한 text label을 제공한다.
- motion 감소 설정에서는 running pulse와 전환 animation을 제거한다.
- Lane palette는 명암 모드별로 별도 token을 사용한다.

## 12. MVP 적용 우선순위

다음 구현 순서에서 이 문법을 검증한다.

1. Lane color token과 Task accent
2. Phase의 색상 면 제거 및 공간 경계 강화
3. 상태를 icon/badge 중심으로 변경
4. 세 Gate 관계선의 형태 분리
5. 선택/focus 명암 조정
6. Gate V0 화면을 다시 판정

이 문법은 Phase 0의 관찰 결과에 따라 바뀔 수 있지만, 변경할 때는 한 시각 채널이 어떤 의미를 소유하는지 함께 갱신한다.
