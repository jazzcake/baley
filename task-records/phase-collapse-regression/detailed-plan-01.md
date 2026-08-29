# Detailed plan — completed Phase collapse regression

- Record ID: `4aee2548-b8e1-495a-b409-93e12626bfd6`
- Task: #153 Completed Phase collapse regression
- Run: `282eb6df-54ca-4f6b-9c6f-a07dde793be1`

## Diagnosis evidence

The completed Phase background has a negative stacking level so it remains
behind graph nodes. Its collapse button was nested in that background and could
not reliably receive pointer input. Separately, selecting a Task in a collapsed
Phase correctly expands it, but the effect also undid an explicit collapse of
the already-selected Task.

## Implementation plan

1. Place the completed-Phase control in a sibling interactive layer above the
   canvas decorations while keeping the Phase background behind nodes.
2. Preserve automatic expansion only for a new Task selection, never for a
   user-initiated collapse of the current selection.
3. Focus the Phase that was toggled, rather than the active Phase.
4. Add a Viewer regression test covering control placement and the collapsed
   layout request, then run focused and full frontend verification.
5. Deploy and smoke-test the completed Phase in the live Viewer before review.
