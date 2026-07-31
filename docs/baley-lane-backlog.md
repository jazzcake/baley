---
type: lane-backlog
status: implemented-live-model
workspace_id: "00000000-0000-4000-8000-000000000001"
workspace_revision_observed: 182
---

# Baley Lane Backlog

Baley now persists lane-scoped `BacklogItem` planning intake separately from
formal Tasks. Items remain phase-free until `backlog.promote` receives an
explicit Phase and relationship intent. The typed command surface, HTTP/CLI/MCP
adapters, PostgreSQL projection, and read-only Viewer use the same B# model.

## Adoption lane

- Existing-project adoption intake: LLM-led inspection of an already-running project's repository and planning evidence, followed by an iterated candidate manifest in chat or a document. When the human says to register the settled set, import the proposed Tasks in one controlled sequence. This must preserve provenance and must never auto-confirm, discard, or close imported work. See `docs/baley-existing-project-intake-guide.md`.
- Multi-repository Task orchestration and multi-repository CommitReference coordination. Deferred explicitly from the current single-repository Adoption slice.
- External notification integrations: Slack and email.
- GitHub and GitLab integration.
- Lane and Gate templates.
- Search and long-term work memory.
- Repository webhooks.
- Release and deployment Gate support.
- External tool import.
- Organization-level capability administration.
- Hosted deployment.

## Client lane

- Real-browser integration regression coverage for React Flow controls, wheel zoom, canvas drag, and Fit continuity. This is the residual verification gap recorded for Task #115.

## Server lane

- Retire the legacy listener on port 8080 after its owning launch context approves the cleanup. This is not an Adoption-slice Task because this workspace does not own that process.

## Promotion rule

Promote a Backlog item into a formal Task only when the target Phase,
acceptance outcome, dependency intent, and successor or terminal reason are
known. Lane, title, and description come from the Backlog item. Promotion uses
the existing Task-create topology/warning contract atomically and never changes
a Gate automatically.

## Existing-project adoption intake protocol

1. The LLM reads repository-local evidence and prepares candidate Tasks and lane Backlog: it owns discovery, lane and phase assignment, dependency design, and the initial acceptance-outcome proposal.
2. The candidate manifest is discussed and revised naturally in chat or a document. It is not a separate approval workflow.
3. Keep uncertain, broad, missing-owner, or deferred work in lane Backlog. Never manufacture implementation status or human confirmations from repository evidence.
4. When the human directs the settled set to be registered, create the proposed Tasks in a validated sequence and write back their public IDs. This ordinary registration direction never authorizes a human-only confirmation, discard, active-Gate change, or Gate pass.
5. After import, show the unresolved Backlog by lane and provide explicit promotion of one or more items into Tasks. Imported Tasks remain pending until normal work, evidence, and human-confirmation rules are satisfied.
