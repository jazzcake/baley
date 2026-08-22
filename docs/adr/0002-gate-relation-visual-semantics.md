# ADR 0002: Gate relation visual semantics

- Status: Accepted
- Date: 2026-08-22
- Scope: Viewer Canvas, Gate relationships

## Context

Gate relations share the Canvas with ordinary Task dependencies. Rendering
`required`, `reference`, and `unlocks` as repeated edge labels made the
relationships difficult to read when the Canvas was zoomed out, while also
adding visual clutter around a Gate's fan-in and fan-out.

The important information is the relationship's direction and present effect:

- a Task may be required before a Gate can pass;
- a Gate may presently prevent a downstream Task from starting;
- after the Gate passes, that same downstream relationship is no longer a
  lock.

## Decision

Canvas Gate edges do not render text labels. Their semantic meaning is carried
by direction, color, and the locked Task endpoint marker.

| Domain relation / state | Direction | Visual treatment |
| --- | --- | --- |
| `required` | Task -> Gate | Solid amber-brown line |
| `reference` | Task -> Gate | Thin neutral gray dashed line |
| `unlocks`, Gate not passed | Gate -> Task | Solid purple line; lock icon at the Task input endpoint |
| `unlocks`, Gate passed | Gate -> Task | Solid teal line; no lock icon |

The lock icon belongs to the target Task, rather than the line midpoint, so it
communicates the Task's current blocked-by-Gate condition at the place where a
user would attempt to follow or start that work.

The Inspector remains the authoritative detailed explanation of a Gate and its
relations. This ADR changes only Canvas presentation; it does not change Gate
state, Task state, relation persistence, or approval rules.

## Consequences

### Positive

- Gate fan-in and fan-out remain legible at low zoom.
- A user can distinguish prerequisite work from currently locked downstream
  work without reading tiny edge labels.
- Passing a Gate visibly changes downstream work from locked to available.
- The same relationship remains stable in the graph while its visual state
  accurately follows the Gate lifecycle.

### Trade-offs

- Exact relation names are not discoverable from an individual Canvas edge;
  users inspect the selected Task or Gate for details.
- Color must not be the sole accessibility channel: direction and the lock
  marker carry the primary distinction for the locked state.
- A collapsed Phase projects several Tasks to a summary endpoint, so a
  per-Task lock marker is visible only after expanding that Phase.

## Verification contract

- No Gate edge label is rendered on the Canvas.
- Required edges point from Task to Gate and use the required color token.
- An unpassed Gate's `unlocks` edge points from Gate to Task, uses the locked
  color token, and renders exactly one lock marker on that Task input.
- After the Gate is passed, its downstream edge uses the unlocked color token
  and the Task lock marker is absent.
- Reference edges remain visually distinct and do not create a Task lock.

## Rejected alternatives

- **Persistent edge labels:** rejected because they become unreadable and
  cluttered at normal overview zoom.
- **Lock at the line midpoint:** rejected because it is ambiguous which Task
  is blocked when multiple Gate edges run nearby.
- **A single color for `unlocks`:** rejected because it cannot distinguish a
  currently locked Task from one released by a passed Gate.
