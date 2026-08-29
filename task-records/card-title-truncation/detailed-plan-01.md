# Detailed plan — card title truncation

- Record ID: `a1a96c9d-1320-4803-93f3-10df06ddf853`
- Task: #154 카드 제목 말줄임 처리
- Run: `9a2f4455-4dd9-4e4b-b715-c1bded759c74`

## Existing implementation evidence

`src/styles.css` applies a two-line WebKit clamp and fixed title height to
`.task-node .task-title`; `TaskNode` emits a development-only structured trace
when a title is clamped. This retains card metadata/status space.

## Verification plan

1. Run the focused TaskNode/Card UI tests and the frontend suite.
2. Build the Viewer production bundle.
3. Confirm the clamp contract remains in the stylesheet and TaskNode trace.
4. Report the existing implementation with this shared evidence; human
   confirmation remains separate.
