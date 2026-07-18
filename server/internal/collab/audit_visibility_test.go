package collab

import (
	"testing"
	"time"

	"github.com/jazzcake/baley/server/internal/authz"
	"github.com/jazzcake/baley/server/internal/domain"
)

func TestBuildAuditTimelineScopesOrdersAndSeparatesActorProvenance(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	approval := &AuditApprovalEvidence{WorkspaceID: "w", ExecutedCommandID: "cmd-2", Attestation: domain.HumanApprovalAttestation{ID: "att", Action: "task_confirm", EntityType: "task", EntityID: "t", WorkspaceRevision: 2, ApprovedByActorID: "human-c", ApprovedCommandHash: "sha256:command"}}
	events := []AuditEvent{
		{
			ID: "2", WorkspaceID: "w", TaskScopes: []AuditTaskScope{{TaskID: "t", LaneID: "l"}}, Command: "task.confirm", CommandID: "cmd-2", CommandHash: "sha256:command", MutationEntityType: "task", MutationEntityID: "t", EventType: "task.confirmed", EntityType: "task", EntityID: "t",
			WorkspaceRevision: 3, InitiatedBy: "human-a", InitiatedKind: authz.ActorHuman, ExecutedBy: "worker-b", ExecutedKind: authz.ActorAgent,
			ApprovedBy: "human-c", ApprovedKind: authz.ActorHuman, ApprovalEvidence: approval, OccurredAt: now,
		},
		{
			ID: "1", WorkspaceID: "w", TaskScopes: []AuditTaskScope{{TaskID: "t", LaneID: "l"}}, Command: "task.update", CommandID: "cmd-1", MutationEntityType: "task", MutationEntityID: "t", EventType: "task.updated", EntityType: "task", EntityID: "t",
			WorkspaceRevision: 2, ExecutedBy: "worker-b", ExecutedKind: authz.ActorAgent, OccurredAt: now.Add(-time.Minute),
		},
		{
			ID: "3", WorkspaceID: "w", LaneIDs: []string{"other"}, Command: "git.observe", CommandID: "cmd-3", MutationEntityType: "observation", MutationEntityID: "o", EventType: "git.observed", EntityType: "observation", EntityID: "o",
			WorkspaceRevision: 4, ExecutedBy: "system", ExecutedKind: authz.ActorSystem, OccurredAt: now.Add(time.Minute),
		},
	}
	values, err := BuildAuditTimeline(AuditQuery{WorkspaceID: "w", LaneID: "l"}, events)
	if err != nil || len(values) != 2 || values[0].ID != "2" {
		t.Fatalf("audit projection wrong: %+v %v", values, err)
	}
	if !values[0].ApprovalRequired || !values[0].ApprovalRecorded || !values[0].AgentExecuted {
		t.Fatalf("approval/agent distinction missing: %+v", values[0])
	}
	if !values[1].InitiatedBy.Missing || !values[1].ApprovedBy.Missing || values[1].ApprovalRequired || values[1].ApprovalRecorded {
		t.Fatalf("missing/non-approval provenance wrong: %+v", values[1])
	}
}

func TestBuildAuditTimelineRequiresBoundApprovalEvidence(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	event := AuditEvent{
		ID: "event", WorkspaceID: "w", TaskScopes: []AuditTaskScope{{TaskID: "t", LaneID: "l"}}, Command: "task.confirm", CommandID: "cmd", CommandHash: "sha256:command", MutationEntityType: "task", MutationEntityID: "t",
		EventType: "task.confirmed", EntityType: "task", EntityID: "t", WorkspaceRevision: 3, ExecutedBy: "agent", ExecutedKind: authz.ActorAgent,
		ApprovedBy: "human", ApprovedKind: authz.ActorHuman, OccurredAt: now,
	}
	values, err := BuildAuditTimeline(AuditQuery{WorkspaceID: "w"}, []AuditEvent{event})
	if err != nil || !values[0].ApprovalRequired || values[0].ApprovalRecorded {
		t.Fatalf("actor ID alone counted as approval: %+v %v", values, err)
	}
	event.ApprovalEvidence = &AuditApprovalEvidence{WorkspaceID: "w", ExecutedCommandID: "cmd", Attestation: domain.HumanApprovalAttestation{ID: "att", Action: "task_confirm", EntityType: "task", EntityID: "other", WorkspaceRevision: 2, ApprovedByActorID: "human", ApprovedCommandHash: "sha256:command"}}
	values, err = BuildAuditTimeline(AuditQuery{WorkspaceID: "w"}, []AuditEvent{event})
	if err != nil || values[0].ApprovalRecorded {
		t.Fatalf("mismatched evidence counted as approval: %+v %v", values, err)
	}
}

func TestBuildAuditTimelineRequiresOwnerAndGateDecisionEvidence(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	workspaceApproval := &AuditApprovalEvidence{WorkspaceID: "w", ExecutedCommandID: "cmd-close", Attestation: domain.HumanApprovalAttestation{ID: "att", Action: "workspace_close", EntityType: "workspace", EntityID: "w", WorkspaceRevision: 4, ApprovedByActorID: "human", ApprovedCommandHash: "sha256:close", ApproverRole: "approver"}}
	workspaceEvent := AuditEvent{ID: "close", WorkspaceID: "w", Command: "workspace.close", CommandID: "cmd-close", CommandHash: "sha256:close", MutationEntityType: "workspace", MutationEntityID: "w", EventType: "workspace.closed", EntityType: "workspace", EntityID: "w", WorkspaceRevision: 5, ExecutedBy: "human", ExecutedKind: authz.ActorHuman, ApprovedBy: "human", ApprovedKind: authz.ActorHuman, ApprovalEvidence: workspaceApproval, OccurredAt: now}
	values, err := BuildAuditTimeline(AuditQuery{WorkspaceID: "w"}, []AuditEvent{workspaceEvent})
	if err != nil || values[0].ApprovalRecorded {
		t.Fatalf("non-owner workspace approval accepted: %+v %v", values, err)
	}
	workspaceApproval.Attestation.ApproverRole = "owner"
	values, err = BuildAuditTimeline(AuditQuery{WorkspaceID: "w"}, []AuditEvent{workspaceEvent})
	if err != nil || !values[0].ApprovalRecorded {
		t.Fatalf("owner workspace approval rejected: %+v %v", values, err)
	}

	gateApproval := &AuditApprovalEvidence{WorkspaceID: "w", ExecutedCommandID: "cmd-gate", Attestation: domain.HumanApprovalAttestation{ID: "gate-att", Action: "gate_pass", EntityType: "gate", EntityID: "g", WorkspaceRevision: 7, ApprovedByActorID: "human", ApprovedCommandHash: "sha256:gate", DecisionSnapshotHash: "sha256:decision"}}
	gateEvent := AuditEvent{ID: "gate", WorkspaceID: "w", Command: "gate.pass", CommandID: "cmd-gate", CommandHash: "sha256:gate", MutationEntityType: "gate", MutationEntityID: "g", EventType: "gate.passed", EntityType: "gate", EntityID: "g", WorkspaceRevision: 8, ExecutedBy: "agent", ExecutedKind: authz.ActorAgent, ApprovedBy: "human", ApprovedKind: authz.ActorHuman, ApprovalEvidence: gateApproval, OccurredAt: now}
	values, err = BuildAuditTimeline(AuditQuery{WorkspaceID: "w"}, []AuditEvent{gateEvent})
	if err != nil || values[0].ApprovalRecorded {
		t.Fatalf("gate approval without decision binding accepted: %+v %v", values, err)
	}
	gateEvent.DecisionSnapshotHash = "sha256:decision"
	values, err = BuildAuditTimeline(AuditQuery{WorkspaceID: "w"}, []AuditEvent{gateEvent})
	if err != nil || !values[0].ApprovalRecorded {
		t.Fatalf("bound gate approval rejected: %+v %v", values, err)
	}
}

func TestBuildAuditTimelineRejectsCommandEventAndScopeMismatch(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	event := AuditEvent{ID: "e", WorkspaceID: "w", TaskScopes: []AuditTaskScope{{TaskID: "t", LaneID: "l"}}, Command: "task.update", CommandID: "cmd", MutationEntityType: "task", MutationEntityID: "t", EventType: "task.confirmed", EntityType: "task", EntityID: "t", WorkspaceRevision: 2, ExecutedBy: "human", ExecutedKind: authz.ActorHuman, OccurredAt: now}
	if _, err := BuildAuditTimeline(AuditQuery{WorkspaceID: "w"}, []AuditEvent{event}); err == nil {
		t.Fatal("command/event mismatch accepted")
	}
	event.EventType = "task.updated"
	event.TaskScopes = []AuditTaskScope{{TaskID: "other", LaneID: "l"}}
	if _, err := BuildAuditTimeline(AuditQuery{WorkspaceID: "w"}, []AuditEvent{event}); err == nil {
		t.Fatal("task entity/scope mismatch accepted")
	}
	event.TaskScopes = []AuditTaskScope{{TaskID: "t", LaneID: "l"}, {TaskID: "other", LaneID: "l"}}
	event.EntityID = "other"
	if _, err := BuildAuditTimeline(AuditQuery{WorkspaceID: "w"}, []AuditEvent{event}); err == nil {
		t.Fatal("primary event target different from mutation target accepted")
	}
}

func TestBuildAuditTimelineBindsAttestationToWorkspaceAndExecutedCommand(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	evidence := &AuditApprovalEvidence{WorkspaceID: "other", ExecutedCommandID: "cmd", Attestation: domain.HumanApprovalAttestation{ID: "att", Action: "task_confirm", EntityType: "task", EntityID: "t", WorkspaceRevision: 2, ApprovedByActorID: "human", ApprovedCommandHash: "sha256:command"}}
	event := AuditEvent{ID: "e", WorkspaceID: "w", TaskScopes: []AuditTaskScope{{TaskID: "t", LaneID: "l"}}, Command: "task.confirm", CommandID: "cmd", CommandHash: "sha256:command", MutationEntityType: "task", MutationEntityID: "t", EventType: "task.confirmed", EntityType: "task", EntityID: "t", WorkspaceRevision: 3, ExecutedBy: "agent", ExecutedKind: authz.ActorAgent, ApprovedBy: "human", ApprovedKind: authz.ActorHuman, ApprovalEvidence: evidence, OccurredAt: now}
	values, err := BuildAuditTimeline(AuditQuery{WorkspaceID: "w"}, []AuditEvent{event})
	if err != nil || values[0].ApprovalRecorded {
		t.Fatalf("cross-workspace evidence accepted: %+v %v", values, err)
	}
	evidence.WorkspaceID = "w"
	evidence.ExecutedCommandID = "other-command"
	values, err = BuildAuditTimeline(AuditQuery{WorkspaceID: "w"}, []AuditEvent{event})
	if err != nil || values[0].ApprovalRecorded {
		t.Fatalf("cross-command evidence accepted: %+v %v", values, err)
	}
}

func TestBuildAuditTimelineSupportsMultiTaskScopeAndInterleavedOrdering(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	events := []AuditEvent{
		{ID: "a", WorkspaceID: "w", Command: "git.observe", CommandID: "cmd-a", MutationEntityType: "observation", MutationEntityID: "o", EventType: "git.observed", EntityType: "observation", EntityID: "o", WorkspaceRevision: 1, ExecutedBy: "system", ExecutedKind: authz.ActorSystem, OccurredAt: now},
		{ID: "c", WorkspaceID: "w", TaskScopes: []AuditTaskScope{{TaskID: "t1", LaneID: "l1"}, {TaskID: "t2", LaneID: "l2"}}, Command: "dependency.connect", CommandID: "cmd-c", MutationEntityType: "workspace_graph", MutationEntityID: "workspace_graph", EventType: "dependency.connected", EntityType: "workspace_graph", EntityID: "workspace_graph", WorkspaceRevision: 3, ExecutedBy: "human-2", ExecutedKind: authz.ActorHuman, OccurredAt: now},
		{ID: "b", WorkspaceID: "w", TaskScopes: []AuditTaskScope{{TaskID: "t1", LaneID: "l1"}}, Command: "task.block", CommandID: "cmd-b", MutationEntityType: "task", MutationEntityID: "t1", EventType: "task.blocked", EntityType: "task", EntityID: "t1", WorkspaceRevision: 2, ExecutedBy: "human-1", ExecutedKind: authz.ActorHuman, OccurredAt: now},
	}
	values, err := BuildAuditTimeline(AuditQuery{WorkspaceID: "w", TaskID: "t2", ImportantOnly: true}, events)
	if err != nil || len(values) != 1 || values[0].ID != "c" {
		t.Fatalf("multi-task audit scope wrong: %+v %v", values, err)
	}
	values, err = BuildAuditTimeline(AuditQuery{WorkspaceID: "w", ImportantOnly: true}, events)
	if err != nil || len(values) != 2 || values[0].ID != "c" || values[1].ID != "b" {
		t.Fatalf("important audit filter/order wrong: %+v %v", values, err)
	}
}

func TestBuildAuditTimelineActiveGateApprovalAndInvalidExecutor(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	event := AuditEvent{ID: "event", WorkspaceID: "w", TaskScopes: []AuditTaskScope{{TaskID: "t", LaneID: "l"}}, Command: "gate.attach_task", CommandID: "cmd", MutationEntityType: "gate", MutationEntityID: "g", EventType: "gate.task_attached", EntityType: "gate", EntityID: "g", WorkspaceRevision: 1, ExecutedBy: "operator", ExecutedKind: authz.ActorHuman, OccurredAt: now}
	withoutActiveGate, err := BuildAuditTimeline(AuditQuery{WorkspaceID: "w"}, []AuditEvent{event})
	if err != nil || withoutActiveGate[0].ApprovalRequired {
		t.Fatalf("inactive gate unexpectedly required approval: %+v %v", withoutActiveGate, err)
	}
	event.ActiveGate = true
	withActiveGate, err := BuildAuditTimeline(AuditQuery{WorkspaceID: "w"}, []AuditEvent{event})
	if err != nil || !withActiveGate[0].ApprovalRequired || withActiveGate[0].ApprovalRecorded || !withActiveGate[0].ApprovedBy.Missing {
		t.Fatalf("active gate approval evidence wrong: %+v %v", withActiveGate, err)
	}
	event.ExecutedKind = ""
	if _, err := BuildAuditTimeline(AuditQuery{WorkspaceID: "w"}, []AuditEvent{event}); err == nil {
		t.Fatal("executor without kind accepted")
	}
}

func TestAuditImportanceClassifiesEveryDomainEvent(t *testing.T) {
	if len(auditEventImportance) != len(domain.EventEvidenceRules) {
		t.Fatalf("importance catalog size %d, event catalog size %d", len(auditEventImportance), len(domain.EventEvidenceRules))
	}
	for _, rule := range domain.EventEvidenceRules {
		if _, exists := auditEventImportance[rule.EventType]; !exists {
			t.Fatalf("event %q has no importance decision", rule.EventType)
		}
	}
	for _, eventType := range []string{"run.started", "run.succeeded", "run.failed", "run.cancelled", "run.interrupted", "run.corrected"} {
		if !auditEventImportance[eventType] {
			t.Fatalf("run lifecycle event %q is not important", eventType)
		}
	}
}

func TestAuditCommandBindingAllowsDeclaredSecondaryEvents(t *testing.T) {
	for command, eventTypes := range secondaryAuditEvents {
		for _, eventType := range eventTypes {
			if !auditCommandAllowsEvent(command, eventType, command == "gate.pass") {
				t.Fatalf("declared secondary %s -> %s rejected", command, eventType)
			}
		}
	}
	for _, policy := range domain.MutationPolicies {
		if policy.OperationalNoEvent {
			continue
		}
		if !auditCommandAllowsEvent(policy.Name, policy.EventType, false) {
			t.Fatalf("primary event %s -> %s rejected", policy.Name, policy.EventType)
		}
	}
}

func TestBuildAuditTimelineRequiresCommandGroupProvenanceConsistency(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	events := []AuditEvent{
		{ID: "workspace-event", WorkspaceID: "w", Command: "workspace.activate", CommandID: "cmd", MutationEntityType: "workspace", MutationEntityID: "w", EventType: "workspace.activated", EntityType: "workspace", EntityID: "w", WorkspaceRevision: 2, ExecutedBy: "agent-b", ExecutedKind: authz.ActorAgent, OccurredAt: now},
		{ID: "phase-event", WorkspaceID: "w", Command: "workspace.activate", CommandID: "cmd", MutationEntityType: "workspace", MutationEntityID: "w", EventType: "phase.activated", EntityType: "phase", EntityID: "p", WorkspaceRevision: 2, ExecutedBy: "human-d", ExecutedKind: authz.ActorHuman, OccurredAt: now},
	}
	if _, err := BuildAuditTimeline(AuditQuery{WorkspaceID: "w"}, events); err == nil {
		t.Fatal("same command with inconsistent executor provenance accepted")
	}
	events[1].ExecutedBy, events[1].ExecutedKind = events[0].ExecutedBy, events[0].ExecutedKind
	values, err := BuildAuditTimeline(AuditQuery{WorkspaceID: "w"}, events)
	if err != nil || len(values) != 2 || values[0].ExecutedBy != values[1].ExecutedBy {
		t.Fatalf("consistent command group rejected: %+v %v", values, err)
	}
}
