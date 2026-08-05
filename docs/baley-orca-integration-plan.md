---
type: plan
status: draft-for-team-review
scope: orca-integration-proposal
last_active: 2026-08-05
when_to_read: "Orca를 주 작업 IDE로 사용하면서 Baley의 Lane·Task·Run·증거 관리와 연결하는 방안을 검토할 때"
affects:
  - docs/baley-system-spec-v1.md
  - docs/baley-command-architecture.md
  - docs/baley-roadmap.md
  - contracts/v1
  - .agents/skills/baley-manage-work/SKILL.md
---

# Orca를 주 작업 IDE로 사용하기 위한 Baley 연동 제안

## 1. 이 문서의 성격

이 문서는 Baley의 내부 모델이나 상태 전이를 확정하는 설계 스펙이 아니다. Orca를 주 작업 IDE로 사용하려는 입장에서, 우리가 이해한 두 제품의 강점과 결합 가설을 Baley 팀에 전달하기 위한 제안서다.

Baley의 Task, Lane, Run, 권한, command와 증거 모델은 Baley 팀이 가장 잘 알고 있다. 따라서 이 문서에서 제시하는 데이터 위치, 상태 연결과 UI 흐름은 모두 검토용 아이디어다. Baley의 기존 원칙과 구현 제약에 맞는 최종 구조는 Baley 팀이 다시 판단해야 한다.

다만 실제 연동에서 지켜야 할 핵심 관계와 중복 실행 방지 조건은 명확히 전달한다.

## 2. 제안 배경

Orca는 실제 작업을 수행하는 주 작업 환경으로 사용할 수 있다. 사용자는 Orca에서 task를 만들고, 격리된 작업공간에서 Agent와 작업하며, 터미널·코드 변경·실행 결과를 한 흐름으로 다룬다.

반면 장기 프로젝트에서는 “지금 어떤 작업을 실행 중인가”만으로 충분하지 않다. 여러 업무 전선의 목적, 선후 관계, 단계, 승인, 중단 후 복원과 최종 증거를 지속적으로 관리해야 한다. 우리가 이해한 Baley의 강점은 이 장기 업무 구조를 Lane과 Task 중심으로 보존하는 데 있다.

따라서 한 제품이 다른 제품을 대체하기보다 다음과 같이 역할을 나누는 방식을 제안한다.

```text
Baley: 무엇을 왜 진행하며, 어떤 관계와 근거로 완료되는가
Orca: 지금 선택한 작업을 어디에서 어떻게 실행하는가
```

## 3. 우리가 이해한 Baley

현재 문서와 사용 경험을 기준으로 이해한 Baley는 다음과 같다.

- **Lane**은 일정 기간 지속되는 독립적인 업무 전선이다.
- **Task**는 Lane과 Phase에 놓이며 관계, 현재 맥락과 lifecycle을 가진다.
- **Run**은 상세 계획, 구현, 리뷰와 보고 같은 실제 작업 시도를 추적한다.
- **Task Record와 Git evidence**는 작업 과정과 결과를 장기적으로 복원하는 근거다.
- **Gate와 인간 승인 경계**는 Agent 실행과 사람의 최종 판단을 분리한다.
- **Baley Viewer**는 정본 상태와 관계를 읽는 화면이며, command 경로가 변경을 담당한다.
- Baley는 로컬 worktree를 직접 관리하는 IDE가 아니며 branch와 worktree는 비권위적 관찰로 취급한다.

이 이해가 부정확한 부분은 Baley 팀의 설명을 우선한다. 특히 Orca execution을 기존 Run에 어떻게 연결할지는 이 문서에서 결정하지 않는다.

## 4. Orca는 어떤 역할을 하는가

이 제안에서 Orca는 Baley의 대체재가 아니라 **실제 실행이 일어나는 주 작업 IDE**다.

현재 전제하는 Orca의 사용 특성은 다음과 같다.

- 사용자가 작업 단위인 Orca task를 생성하고 다시 열 수 있다.
- 각 task는 고유 ID를 가지며 외부에서 현재 실행을 식별할 수 있다.
- task별 작업공간 또는 worktree에서 코드를 변경하고 터미널을 실행할 수 있다.
- 한 논리 작업을 실패·포기하거나 결과를 검토한 뒤 새 task로 다시 시도할 수 있다.
- 실행이 끝난 뒤 코드 변경, commit, 요약과 관찰 결과를 남길 수 있다.
- 사용자는 계획 시스템과 IDE를 오가기보다 Orca에서 실제 작업을 계속 수행하는 경험을 원한다.

Orca의 구체적인 API, event, heartbeat, idempotency와 task 복구 기능은 아직 이 문서에서 확정하지 않는다. 실제 연동 설계 전에 Orca가 제공하는 인터페이스를 별도로 확인해야 한다.

## 5. 결합 목표

목표는 **Baley에서 관리되는 장기 업무 맥락을 잃지 않으면서 Orca를 일상적인 작업 진입점으로 사용하는 것**이다.

사용자가 기대하는 흐름은 다음과 같다.

1. Baley에서 지금 진행할 Lane의 Task를 선택한다.
2. 해당 Task를 Orca task로 실행한다.
3. 이후 계획, 구현, 터미널 작업과 검증은 Orca에서 수행한다.
4. Orca를 닫거나 작업이 중단되어도 Baley에서 현재 실행과 기존 시도를 찾을 수 있다.
5. 결과를 Baley의 Task, Run, Task Record와 Git evidence에 연결한다.
6. 최종 완료 확인과 Gate 판단은 기존 Baley 권한 경계를 따른다.

이 흐름이 성립하면 Baley는 IDE가 되지 않고도 실행 현황을 잃지 않으며, Orca는 별도의 장기 프로젝트 관리 기능을 중복 구현하지 않고 주 작업 환경에 집중할 수 있다.

## 6. 제품 가설

### 가설 1: 계획과 실행 환경을 분리하되 연결하면 전환 비용이 줄어든다

Baley가 장기 맥락과 다음 작업을 제공하고 Orca가 선택된 작업을 바로 열어 준다면, 사용자는 Task 내용을 다시 복사하거나 작업공간을 수동으로 찾는 시간을 줄일 수 있다.

### 가설 2: Orca task ID만으로도 유용한 실행 lock을 만들 수 있다

Lane에 현재 Orca task ID가 연결되어 있으면 “이 Lane의 선택된 작업은 이미 Orca에서 실행 중이거나 검토 중”이라는 신호로 사용할 수 있다. 이를 실행 생성 조건에 포함하면 실수로 같은 Lane에서 중복 작업을 시작하는 문제를 줄일 수 있다.

### 가설 3: 현재 실행과 실행 이력을 분리하면 재시도가 자연스럽다

현재 Orca task는 하나만 가리키되, 기각·포기·재생성된 이전 Orca task를 이력으로 유지하면 사용자는 실패한 접근과 남은 Git 결과를 복원할 수 있다.

### 가설 4: 결과를 Baley evidence로 승격해야 장기적으로 가치가 남는다

Orca 세션과 worktree는 일시적일 수 있다. 반면 결과 commit, Task Record, 검증 요약과 채택·기각 근거를 Baley의 기존 증거 체계에 연결하면 실행 환경이 사라져도 판단 근거가 유지된다.

## 7. 핵심 관계 모델

다음 관계는 구현 방식과 별개로 연동의 핵심 조건으로 제안한다.

### 7.1 Orca task의 의미

Orca task는 Baley에서 선택한 **Lane Task의 execution instance**다.

```text
Baley Lane Task
  └─ Orca task = 이 Task를 실제로 수행하는 한 번의 실행 시도
```

Orca task가 새로운 업무 정본을 만드는 것은 아니다. 어떤 목표를 수행하는지는 Baley Task가 정의하고, Orca task는 그 목표를 실행하는 시도다.

### 7.2 논리적 1:1, 물리적 1:N

한 시점의 선택된 Task와 현재 Orca task는 논리적으로 1:1이다.

```text
현재 선택된 Baley Task 1 ── 현재 Orca task 0..1
```

그러나 재시도·기각·포기·삭제 후 재생성을 포함한 전체 이력은 1:N이다.

```text
Baley Task 1 ── Orca execution attempts 0..N
```

새 시도는 이전 Orca task를 덮어쓰거나 되살리지 않고 별도 execution instance로 남기는 편이 복구와 감사에 유리하다고 본다.

### 7.3 Lane의 현재 Orca task ID는 lock이다

Lane에 현재 Orca task ID가 기록되어 있으면 새 Orca task 생성을 막는 lock 효과를 가져야 한다.

```text
current Orca task ID 없음
  → 새 실행 생성 가능 후보

current Orca task ID 있음
  → 해당 Lane의 새 실행 생성 금지
```

이 조건은 UI 표시만이 아니라 실제 생성 경로에서 동시성 안전하게 검증되어야 한다. 다만 ID를 Lane에 직접 둘지, 별도 execution entity를 참조할지, Baley의 기존 Run으로 같은 불변조건을 표현할지는 Baley 팀이 판단할 영역이다.

### 7.4 lock 해제는 명시적인 종결 판단 뒤에 이뤄진다

Orca 실행 프로세스가 끝났다는 사실만으로 즉시 lock을 해제하면 결과 검토 중 중복 실행이 생길 수 있다. 다음 실행을 허용할 수 있을 만큼 현재 시도가 명확히 종결되었을 때 해제하는 것이 안전하다는 의견이다.

구체적으로 어떤 Baley 상태나 command가 종결을 의미하는지는 제안하지 않는다. 채택, 기각, 포기, 생성 실패 복구와 외부 task 소실 등 필요한 상황을 Baley의 기존 모델로 어떻게 표현할지는 재판단이 필요하다.

## 8. 제안하는 사용자 경험

### Baley에서 Orca 작업 시작

- 사용자가 Baley의 Lane에서 실행할 Task를 선택한다.
- Baley의 기존 규칙에 따라 실행 가능한지 확인한다.
- `Orca에서 작업`을 선택하면 Task 맥락을 포함한 Orca task가 생성된다.
- 생성된 Orca task가 자동으로 열리고 사용자는 Orca에서 작업을 이어 간다.
- Baley에는 현재 Orca task ID와 연결된 Task가 보인다.

### Orca에서 작업 지속

- Orca task는 Baley Task ID, 목표, 현재 요약, 다음 행동과 관련 기록을 제공받는다.
- 사용자는 Orca에서 계획, 구현, 터미널 실행과 검증을 수행한다.
- Orca는 Baley의 인간 전용 승인이나 Gate 판단을 대신하지 않는다.
- 필요한 시점에 진행 요약과 Git 결과를 Baley로 전달한다.

### Baley에서 중단 후 복원

- 사용자는 Lane을 열어 현재 연결된 Orca task를 확인한다.
- 해당 task를 Orca에서 다시 열 수 있다.
- 현재 task가 사라졌거나 연결이 불일치하면 즉시 새 task를 만들기보다 기존 실행과 Git 결과를 먼저 확인한다.

### 결과 검토와 다음 시도

- Orca의 결과 요약, 검증, commit과 Task Record를 Baley에서 확인한다.
- 결과가 채택되면 기존 Baley workflow를 통해 구현 보고와 이후 승인을 진행한다.
- 결과가 기각되거나 다시 시도해야 하면 사유를 남기고 현재 연결을 종결한 뒤 새 Orca task를 만든다.
- 이전 시도는 이력에서 계속 확인할 수 있다.

## 9. Orca에 전달하면 유용한 Baley 맥락

실행 시작 시 다음 정보가 있으면 Orca가 주 작업 IDE로 기능하기 쉬울 것으로 본다.

- Workspace, Lane과 Baley Task 식별자
- Task 목표와 설명
- 현재 요약과 next action
- acceptance outcome 또는 완료 판단 기준
- dependency와 blocker 요약
- 관련 Task Record의 정확한 경로와 hash
- repository와 기준 commit
- 허용 범위, 비목표와 인간 확인이 필요한 경계
- Baley에서 현재 실행을 다시 찾을 수 있는 참조

전달 당시의 내용은 execution snapshot으로 고정하는 편이 좋다. 실행 도중 Baley Task가 변경되었을 때 Orca의 작업 지시가 조용히 바뀌면 재현과 리뷰가 어려워지기 때문이다. 변경된 명세는 기존 시도를 종결하고 새 시도에 적용하는 방식을 제안한다.

## 10. Orca에서 Baley로 돌려주면 유용한 정보

- Orca task ID와 현재 관찰 상태
- 실행 시작·마지막 관찰·종료 시각
- 결과 요약과 검증 결과
- 생성하거나 검토한 commit
- Task Record 후보와 파일 hash
- branch hint, worktree label, head commit과 dirty 여부
- 실패·기각·포기 또는 재시도 사유
- 이전 execution과의 관계

worktree 절대 경로나 Orca credential 같은 로컬·비밀 정보는 외부 Baley 서버에 저장하지 않는 기존 원칙이 유지되어야 한다. branch와 worktree 정보도 Task 정본이나 완료 근거가 아니라 비권위적 관찰로 취급하는 것이 적절해 보인다.

## 11. 구현 방향에 대한 의견

다음은 확정 스펙이 아니라 Baley 팀이 구현안을 검토할 때 참고할 수 있는 의견이다.

### 11.1 기존 Run과 연결

Orca task는 실제 실행 시도이므로 Baley Run과 자연스럽게 연결될 가능성이 크다. 예를 들어 Orca task 하나가 구현 Run 하나에 대응할 수 있다. 다만 Orca 작업 안에서 계획·구현·리뷰가 이어질 수 있으므로 항상 1:1로 고정하는 것이 Baley Run 의미와 맞는지는 검토가 필요하다.

가능한 선택지는 다음과 같다.

- Orca task와 Baley Run을 1:1로 연결한다.
- Orca task를 상위 execution으로 두고 여러 Baley Run을 연결한다.
- 별도 execution reference만 추가하고 Run에는 필요한 증거만 연결한다.

### 11.2 현재 실행 참조와 이력 분리

현재 Orca task를 빠르게 찾는 참조와 과거 attempts를 보존하는 이력은 분리하는 편이 좋다. 구현은 Lane 필드, Run metadata 또는 별도 execution entity 중 Baley의 정본 모델에 가장 잘 맞는 방식을 선택할 수 있다.

### 11.3 멱등한 생성

Baley 기록과 Orca task 생성은 서로 다른 시스템에서 일어난다. 네트워크 오류로 응답이 유실되었을 때 같은 Orca task를 찾아 재연결할 수 있도록 생성 요청에 멱등 식별자가 필요할 가능성이 높다.

Orca가 이를 직접 지원하지 않으면 Baley 측 adapter나 로컬 bridge에서 생성 의도와 결과를 보존하는 방식을 검토할 수 있다.

### 11.4 event와 주기적 확인

Orca가 상태 event를 제공한다면 즉시 반영에 사용할 수 있다. 제공하지 않거나 event가 유실될 가능성에 대비해 사용자가 실행 상태를 다시 확인하는 수동 동기화와 제한적인 reconciliation도 필요해 보인다.

### 11.5 deep link

Baley에서 현재 Orca task를 여는 deep link가 있으면 복원 경험이 크게 좋아진다. 링크에는 안전한 task 식별자만 포함하고 credential이나 로컬 절대 경로가 노출되지 않아야 한다.

## 12. 실패와 복구에서 지켜야 할 원칙

구체적인 상태 모델은 Baley 팀 판단에 맡기되 다음 상황은 설계에서 반드시 다뤄야 한다.

- Orca task 생성은 성공했지만 Baley에 ID를 기록하지 못한 경우
- Baley에는 현재 ID가 있지만 Orca task를 찾을 수 없는 경우
- 같은 Lane에서 실행 생성 요청이 동시에 들어온 경우
- 실행 완료 event가 중복되거나 순서가 바뀌어 도착한 경우
- Orca task를 삭제했지만 미커밋 변경이나 아직 연결하지 않은 commit이 남은 경우
- 장시간 관찰되지 않지만 실제로는 아직 실행 중인 경우
- 결과 검토 중 사용자가 새 실행을 요청한 경우

이때 가장 중요한 원칙은 **불일치를 이유로 즉시 새 Orca task를 만들지 않는 것**이다. 먼저 기존 task, execution ID와 Git 결과를 조회해 복구하거나 종결한 뒤 다음 시도를 허용해야 한다.

또한 heartbeat 만료나 일정 시간 경과만으로 lock을 자동 해제하는 것은 위험하다. 실제 실행과 중복될 수 있으므로 stale 후보로 표시하고 조사·복구·명시적 해제 절차를 두는 방식을 제안한다.

## 13. Baley 팀이 재판단해야 할 사항

다음은 의도적으로 이 문서에서 결론 내리지 않는다.

### 도메인 모델

- 이 문서의 “선택된 Lane Task”를 Baley에 별도 개념으로 둘 필요가 있는가
- current Orca task 참조를 Lane, Task, Run 또는 별도 execution 중 어디에 둘 것인가
- Lane 단위 lock이 적절한가, Task 단위 또는 다른 실행 범위가 필요한가
- Orca task와 Baley Run의 cardinality를 어떻게 둘 것인가
- 과거 execution attempts를 어떤 정본 entity로 보존할 것인가

### lifecycle과 권한

- Orca 실행 시작·완료·기각·포기를 기존 command에 어떻게 매핑할 것인가
- 실행 결과 채택과 `task.report_implemented`의 경계를 어디에 둘 것인가
- current 실행 연결의 강제 해제는 어떤 capability와 감사 근거를 요구할 것인가
- Orca가 수행할 수 있는 Operator action의 범위를 어디까지 허용할 것인가

### 동기화와 복구

- Orca가 제공할 API, event와 idempotency 계약은 무엇인가
- stale 후보를 어떤 근거로 표시할 것인가
- 고아 Orca task와 끊어진 Baley reference를 어떻게 재연결할 것인가
- Orca task 삭제 전 Git 변경 보존을 어느 시스템이 확인할 것인가

### UI

- 실행 시작과 현재 Orca task 진입점을 Viewer 또는 별도 command surface 중 어디에 둘 것인가
- lock, 실행 관찰, 결과 검토와 과거 attempts를 어떤 용어로 표시할 것인가
- Baley Viewer의 read-only 원칙을 유지하면서 실행 action을 어떻게 제공할 것인가

## 14. 단계적 실험 제안

처음부터 완전한 양방향 동기화를 만들기보다 다음 순서로 가설을 검증하는 것이 좋다.

### 1단계: 수동 연결

- Baley Task에서 Orca task 생성에 필요한 context bundle을 만든다.
- 생성된 Orca task ID를 Baley에서 현재 실행 참조로 기록한다.
- Baley에서 해당 Orca task를 다시 연다.
- 사용자가 결과 요약과 commit을 수동으로 연결한다.

검증할 것: 이 연결만으로도 context 복사와 작업 복원 비용이 실제로 줄어드는가.

### 2단계: 중복 방지와 실행 이력

- 현재 Orca task ID가 있을 때 새 실행을 거부한다.
- 종결 후 새 attempt를 만들고 이전 시도를 보존한다.
- 끊어진 참조를 조회하고 명시적으로 복구한다.

검증할 것: 중복 실행이 줄고 실패한 접근을 복원하는 데 도움이 되는가.

### 3단계: 결과와 증거 동기화

- Orca 결과 요약, 검증, commit과 Task Record 후보를 Baley로 전달한다.
- 기존 Run과 completion workflow에 연결한다.
- 자동 반영과 사람의 판단 경계를 검증한다.

검증할 것: Orca 작업이 Baley의 지속되는 evidence로 충분히 남는가.

### 4단계: 자동 상태 관찰

- event 또는 reconciliation으로 현재 실행 관찰을 갱신한다.
- stale 후보와 복구 UX를 제공한다.
- 반복적으로 안전성이 확인된 부분만 자동화한다.

## 15. 성공 판단 기준

다음 지표로 결합 가설을 검증할 수 있다.

- Baley Task에서 실제 Orca 작업을 시작하기까지 걸리는 시간
- 중복 Orca task 생성 건수
- 중단 후 현재 작업공간을 복원하는 데 걸리는 시간
- 고아 Orca task와 끊어진 reference 발생 건수
- 실행 결과 중 commit·Task Record·검증 근거가 Baley에 연결된 비율
- 재시도 시 이전 시도의 맥락을 재사용한 비율
- 사용자가 Baley와 Orca 사이에서 수동 복사한 정보의 양
- 잘못된 자동 lock 해제 또는 살아 있는 실행과의 중복 건수

## 16. 기대 효과

이 제안이 성립하면 사용자는 Orca를 주 작업 IDE로 유지하면서도 다음을 얻을 수 있다.

- 어떤 Lane의 어떤 Task를 실행 중인지 지속적으로 확인
- 실행 중복 방지
- Orca task와 작업공간으로 빠른 복귀
- 실패·기각·재시도의 이력 보존
- 일시적인 IDE 세션에서 나온 결과를 Baley의 장기 증거로 승격
- Agent 실행과 사람의 최종 판단 경계 유지

Baley는 worktree 관리자나 IDE가 될 필요가 없고, Orca는 장기 업무 그래프와 승인 시스템을 다시 만들 필요가 없다. 두 제품의 정본 경계를 유지하면서 **Baley가 작업의 방향과 기억을 제공하고 Orca가 실행의 중심이 되는 경험**이 이 제안의 최종 목표다.

## 17. Baley 팀에 요청하는 검토

아래 세 가지를 중심으로 의견을 요청한다.

1. 제안한 핵심 관계인 “Baley Task의 Orca execution instance”, “현재 1:1·이력 1:N”, “Lane의 현재 Orca task ID를 통한 lock”이 Baley의 기존 도메인에 적합한가.
2. 이 관계를 기존 Task·Run·Git observation·Event 모델에 표현하는 가장 자연스러운 방법은 무엇인가.
3. 1단계 수동 연결 실험을 위해 Baley와 Orca 양쪽에 필요한 최소 인터페이스는 무엇인가.

이 검토 결과를 바탕으로 Baley의 정본 문서와 `contracts/v1` 변경 여부를 별도로 결정하는 것이 바람직하다.
