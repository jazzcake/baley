---
type: operator-guideline
status: active
---

# Existing-project Baley Intake Guide

Use this guide when Baley is introduced after a project has already accumulated code, documentation, issues, commits, and informal plans.

## Responsibility boundary

The LLM performs the project analysis. It reads the available evidence, identifies candidate work, proposes lanes, phases, dependencies, outcomes, and the Task-versus-Backlog classification. Baley is the validated destination for the final Task graph; it does not discover or decide the project's work on its own.

The human collaborates by correcting the LLM's draft in chat or a document. Once the draft is settled, a natural instruction such as “올립시다” authorizes registration of that discussed Task set. This is ordinary authorization for `task.create`; it does not authorize any human-only lifecycle or Gate action.

## LLM intake loop

1. Inspect repository-local evidence: project overview, roadmap, user-supplied issue exports, task records, source layout, tests, build configuration, and recent Git history.
2. Produce a candidate intake manifest in chat or a project document. For every row include a stable candidate key, proposed classification, title, outcome, lane, phase, predecessor and successor intent, terminal reason when applicable, source evidence, and confidence or open assumptions.
3. Keep discussing and revising the manifest. The LLM updates its proposed graph instead of asking the human to fill in lane or phase fields.
4. When directed to register it, fresh-read Baley and create the formal Task rows in dependency-safe order. For every additional Task, the LLM must have established an upstream Task; if not, it asks whether the row is intentionally an independent root before registering it. Reconcile assigned public IDs back into the manifest.
5. Preserve the remaining candidates as lane-scoped Backlog. Promote them later only when they have a concrete outcome and graph intent.

## Guardrails

- Repository evidence may inform a proposal but never proves that work is implemented, confirmed, discarded, or approved.
- Do not auto-confirm imported Tasks or infer a human approval from historical commits, documents, or issue state.
- Do not attach an imported Task to an active Gate or pass a Gate without the separate fresh-preview and explicit approval required for those actions.
- Do not invent a terminal reason merely to suppress a topology warning.
- A disconnected component is allowed by the Baley domain, but the LLM must not use it as a fallback for missing context. Only an initial Task or an explicitly independent Task may be registered without a predecessor.
- While Baley lacks a persisted BacklogItem model, retain Backlog in a lane-scoped project document rather than presenting UI placeholder data as authoritative.
