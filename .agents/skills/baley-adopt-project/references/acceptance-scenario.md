# Embedding Enablement acceptance scenario

Run the scenario in a disposable repository and isolated PostgreSQL test
database. Do not point it at the Pilot database.

1. Create a Workspace as Owner, bind a Participant/Operator, create lane,
   phases, and adjacent Gate.
2. Create/order lane Backlog items, promote one to a Task, and create one Task
   directly in a known Phase.
3. Attach a from-Phase condition Task and a distinct to-Phase entry/unlock Task.
   Verify internal Gate ID, `G#`, and alias resolve to the same Gate.
4. Expire a Run lease, open a fresh service/session, sweep the Run to
   `interrupted`, and recover through the read-only lane brief.
5. Create a real Git/Record mismatch and verify lane-brief reads do not change
   Workspace revision, Events, or command count.
6. Submit typed evidence for a delegated technical Task and verify only that
   Task auto-confirms.
7. Verify human-required Task and Gate/Lane/Workspace actions do not execute
   without authenticated human approval.
8. Inspect mutation-attempt audit data for stable digests and absence of raw
   approval text, tokens, and passwords. Do not use a secret as idempotency key.
9. Produce and validate a `pilot-measurement` Record.

The acceptance result is PASS only when all focused suites, the aggregate
script, full Go tests and vet, frontend tests/build, skill validation, and an
independent review have no blocking findings.
