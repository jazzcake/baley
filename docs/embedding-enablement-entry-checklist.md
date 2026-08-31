---
type: embedding-enablement-entry-checklist
status: reviewed-contract-baseline
source_task: 120
approval_authority_superseded_by: docs/task-acceptance-policy-contract.md
---

> Historical checklist: delegated acceptance and auto-confirm are no longer
> supported. Every Task remains human-required.

# Embedding Enablement entry checklist

This checklist is the hand-off from the Embedding Contract phase to implementation.
It confirms the contract baseline; it does not pass the Gate.

## Contract baseline

- #117 defines scope, human-authority boundaries, and the split between the current
  V1 human-confirm path and the future delegated policy implementation.
- #118 fixes an immutable acceptance binding on every created Task. A human-approved
  policy change affects only future-Task templates. Task #130 owns the runtime,
  storage, transport, and Viewer implementation of that future policy.
- #119 defines evidence precedence, active-Run-first/read-only recovery,
  `reporting_pending`, mismatch handling, and reproducible Pilot measurements.
- Gate, Gate Task pass/revoke, Lane closure, Workspace closure, discard, and active
  Gate-condition changes remain separate human decision boundaries.

## Enablement implementation map

| Task | Required delivered outcome | Entry rule |
| --- | --- | --- |
| #121 | Existing operator/intake vertical slice is completed and its privileged runtime activation is verified | Continue its recorded next action; do not treat this checklist as proof of activation |
| #122 | Lane brief and evidence-recovery paths use the #119 trust and mismatch rules | Implement after the Contract Gate passes |
| #129 | Gate entry/unlock binding model is reconciled with the phase topology | Preserve its independent scope; do not invent a successor edge here |
| #130 | Delegated Task acceptance policy is implemented exactly to #118 | Only this Task may introduce auto-confirm; it cannot widen Gate/Lane/Workspace authority |
| #123 | Pilot kit uses the above runtime and recovery contracts | Depends on #130 as recorded in the Adoption manifest |
| #124 | Isolated end-to-end acceptance produces independent review and residual-risk evidence | Executes after the Enablement components are ready |

## Gate decision evidence

Before a human considers passing `embedding-enablement-entry`, verify:

1. Tasks #117, #118, #119, and #120 are confirmed.
2. This checklist has no unresolved contract contradiction.
3. The Enablement Tasks remain in their intended phase and no active Gate condition
   has been silently changed.
4. The decision brief reports the scope being unlocked and the retained human-only
   authority boundaries.

Passing the Gate is not implied by completion of this checklist; it needs its own
fresh preview and explicit human approval.
