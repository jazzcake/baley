package domain

import "testing"

func TestMutationPoliciesHaveEventsExceptHeartbeat(t *testing.T) {
	rules := map[string]bool{}
	for _, rule := range EventEvidenceRules {
		rules[rule.EventType] = true
	}
	for _, policy := range MutationPolicies {
		if policy.OperationalNoEvent {
			if policy.Name != "run.heartbeat" || policy.EventType != "" {
				t.Errorf("invalid operational exception: %+v", policy)
			}
			continue
		}
		if policy.EventType == "" {
			t.Errorf("mutation %s has no Event", policy.Name)
		}
		if !rules[policy.EventType] {
			t.Errorf("mutation %s Event %s has no evidence rule", policy.Name, policy.EventType)
		}
	}
}

func TestValidateEventEvidenceRequiresMinimumPayload(t *testing.T) {
	valid := PlannedEvent{Type: "run.started", EntityType: "run", EntityID: "run", Payload: map[string]any{"runId": "run", "taskId": "task", "clientRunId": "client", "kind": "implementation"}}
	if evaluation := ValidateEventEvidence(valid); evaluation.HasErrors() {
		t.Fatalf("valid evidence rejected: %+v", evaluation)
	}
	delete(valid.Payload, "taskId")
	if evaluation := ValidateEventEvidence(valid); !evaluation.HasErrors() {
		t.Fatal("missing evidence accepted")
	}
}

func TestValidateCommandAuditRequiresDomainAndApprovalEvents(t *testing.T) {
	domainEvent := PlannedEvent{Type: "task.confirmed", EntityType: "task", EntityID: "task", Payload: map[string]any{"taskId": "task"}}
	expectation := AuditExpectation{Command: "task.confirm", EntityType: "task", EntityID: "task", WorkspaceRevision: 7, CommandHash: "sha256:x"}
	approval := PlannedEvent{Type: "human_approval_attestation.recorded", EntityType: "attestation", EntityID: "att", Payload: map[string]any{"action": "task_confirm", "entityType": "task", "entityId": "task", "workspaceRevision": 7, "approvedByActorId": "human", "approvedCommandHash": "sha256:x"}}
	if evaluation := ValidateCommandAudit(expectation, []PlannedEvent{domainEvent, approval}, ActorProvenance{ExecutedBy: "agent", ApprovedBy: "human"}); evaluation.HasErrors() {
		t.Fatalf("valid audit rejected: %+v", evaluation)
	}
	if evaluation := ValidateCommandAudit(expectation, []PlannedEvent{domainEvent}, ActorProvenance{ExecutedBy: "agent"}); !hasDiagnostic(evaluation.Errors, CodeHumanApprovalRequired) {
		t.Fatalf("approval omission accepted: %+v", evaluation)
	}
	if evaluation := ValidateCommandAudit(AuditExpectation{Command: "run.heartbeat"}, nil, ActorProvenance{ExecutedBy: "agent"}); evaluation.HasErrors() {
		t.Fatalf("heartbeat exception rejected: %+v", evaluation)
	}
}

func TestProjectGatePassEvidencePreservesHistoricalBasis(t *testing.T) {
	evidence, err := ProjectGatePassEvidence("gate", 8, "attestation", []GateTaskCondition{{GateID: "gate", LinkID: "l2", TaskID: "t2", TaskStatus: TaskImplemented, Passed: true, PassReason: "waived"}, {GateID: "gate", LinkID: "l1", TaskID: "t1", TaskStatus: TaskConfirmed}})
	if err != nil || evidence.WorkspaceRevision != 8 || evidence.HumanApprovalAttestationID != "attestation" || len(evidence.Conditions) != 2 || evidence.Conditions[0].TaskStatus != TaskConfirmed || !evidence.Conditions[1].Passed || evidence.Conditions[1].PassReason != "waived" {
		t.Fatalf("evidence lost: %+v", evidence)
	}
}

func TestProjectGatePassEvidenceRejectsUnsatisfiedCondition(t *testing.T) {
	_, err := ProjectGatePassEvidence("gate", 8, "attestation", []GateTaskCondition{{GateID: "gate", LinkID: "link", TaskID: "task", TaskStatus: TaskPending}})
	if err == nil {
		t.Fatal("unsatisfied Gate condition projected as pass evidence")
	}
}

func TestEventEvidenceFailsClosedForUnknownTypeAndEmptyEntity(t *testing.T) {
	for _, event := range []PlannedEvent{{Type: "unknown", EntityType: "task", EntityID: "task", Payload: map[string]any{}}, {Type: "task.confirmed", EntityType: "task", Payload: map[string]any{"taskId": "task"}}} {
		if evaluation := ValidateEventEvidence(event); !evaluation.HasErrors() {
			t.Fatalf("invalid event accepted: %+v", event)
		}
	}
}

func TestImplementedEvidenceAllowsExplicitEmptyWarningSnapshots(t *testing.T) {
	event := PlannedEvent{Type: "task.implemented_reported", EntityType: "task", EntityID: "task", Payload: map[string]any{
		"taskId": "task", "assessment": "done", "warnings": []string{}, "acknowledgedWarningCodes": []string{},
	}}
	if evaluation := ValidateEventEvidence(event); evaluation.HasErrors() {
		t.Fatalf("explicit empty warning snapshots rejected: %+v", evaluation)
	}
}

func TestImplementedEvidenceBindsWarningsToAcknowledgementAndReason(t *testing.T) {
	base := PlannedEvent{Type: "task.implemented_reported", EntityType: "task", EntityID: "task", Payload: map[string]any{
		"taskId": "task", "assessment": "done", "warnings": []string{CodeDanglingPath}, "acknowledgedWarningCodes": []string{CodeDanglingPath}, "proceedReason": "intentional leaf",
	}}
	if evaluation := ValidateEventEvidence(base); evaluation.HasErrors() {
		t.Fatalf("bound warning evidence rejected: %+v", evaluation)
	}
	base.Payload["acknowledgedWarningCodes"] = []string{CodeMissingDetailedPlan}
	if evaluation := ValidateEventEvidence(base); !evaluation.HasErrors() {
		t.Fatal("mismatched warning acknowledgement accepted")
	}
}

func TestGatePassEvidenceRequiresNonEmptyConditions(t *testing.T) {
	event := PlannedEvent{Type: "gate.passed", EntityType: "gate", EntityID: "gate", Payload: map[string]any{
		"gateId": "gate", "conditions": []GatePassConditionEvidence{}, "humanApprovalAttestationId": "att", "workspaceRevision": 3, "decisionSnapshotHash": "sha256:snapshot",
	}}
	if evaluation := ValidateEventEvidence(event); !evaluation.HasErrors() {
		t.Fatal("empty gate condition snapshot accepted")
	}
}

func TestGateDecisionSnapshotHashChangesWithConditionOrRevision(t *testing.T) {
	workspace := Workspace{ID: "workspace", Revision: 4}
	gate := Gate{ID: "gate", FromPhaseID: "p1", ToPhaseID: "p2", CriteriaRevision: 2}
	conditions := []GateTaskCondition{{GateID: gate.ID, LinkID: "link", TaskID: "task", TaskStatus: TaskConfirmed}}
	base := GateDecisionSnapshotHash(workspace, gate, conditions)
	workspace.Revision++
	if base == GateDecisionSnapshotHash(workspace, gate, conditions) {
		t.Fatal("Workspace revision did not affect Gate decision snapshot")
	}
	workspace.Revision--
	conditions[0].Passed, conditions[0].PassReason = true, "waived"
	waived := GateDecisionSnapshotHash(workspace, gate, conditions)
	if base == waived {
		t.Fatal("condition evidence did not affect Gate decision snapshot")
	}
	conditions[0].PassReason = "waived for a different reason"
	if waived == GateDecisionSnapshotHash(workspace, gate, conditions) {
		t.Fatal("pass reason content did not affect Gate decision snapshot")
	}
}

func TestApprovalCannotBeReusedAcrossCommandActorOrEntity(t *testing.T) {
	domainEvent := PlannedEvent{Type: "task.confirmed", EntityType: "task", EntityID: "task", Payload: map[string]any{"taskId": "task"}}
	approval := PlannedEvent{Type: "human_approval_attestation.recorded", EntityType: "attestation", EntityID: "att", Payload: map[string]any{"action": "task_confirm", "entityType": "task", "entityId": "other", "workspaceRevision": 7, "approvedByActorId": "other-human", "approvedCommandHash": "sha256:wrong"}}
	evaluation := ValidateCommandAudit(AuditExpectation{Command: "task.confirm", EntityType: "task", EntityID: "task", WorkspaceRevision: 7, CommandHash: "sha256:right"}, []PlannedEvent{domainEvent, approval}, ActorProvenance{ExecutedBy: "agent", ApprovedBy: "human"})
	if !hasDiagnostic(evaluation.Errors, CodeHumanApprovalMismatch) {
		t.Fatalf("mismatched approval accepted: %+v", evaluation)
	}
}

func TestActiveGateAttachmentApprovalBindsGateRevisionActorAndHash(t *testing.T) {
	domainEvent := PlannedEvent{Type: "gate.task_attached", EntityType: "gate", EntityID: "gate", Payload: map[string]any{"gateId": "gate", "taskId": "task", "criteriaRevisionAfter": 2}}
	approval := PlannedEvent{Type: "human_approval_attestation.recorded", EntityType: "attestation", EntityID: "att", Payload: map[string]any{
		"action": "gate_attach_task", "entityType": "gate", "entityId": "gate", "workspaceRevision": 9, "approvedByActorId": "owner", "approvedCommandHash": "sha256:command",
	}}
	expectation := AuditExpectation{Command: "gate.attach_task", EntityType: "gate", EntityID: "gate", WorkspaceRevision: 9, CommandHash: "sha256:command", ActiveGate: true}
	if evaluation := ValidateCommandAudit(expectation, []PlannedEvent{domainEvent, approval}, ActorProvenance{ExecutedBy: "agent", ApprovedBy: "owner"}); evaluation.HasErrors() {
		t.Fatalf("bound active Gate approval rejected: %+v", evaluation)
	}
	expectation.WorkspaceRevision++
	if evaluation := ValidateCommandAudit(expectation, []PlannedEvent{domainEvent, approval}, ActorProvenance{ExecutedBy: "agent", ApprovedBy: "owner"}); !hasDiagnostic(evaluation.Errors, CodeHumanApprovalMismatch) {
		t.Fatalf("stale active Gate approval accepted: %+v", evaluation)
	}
}

func TestCommandAuditRejectsRightEventTypeForWrongEntity(t *testing.T) {
	event := PlannedEvent{Type: "task.confirmed", EntityType: "task", EntityID: "other", Payload: map[string]any{"taskId": "other"}}
	expectation := AuditExpectation{Command: "task.confirm", EntityType: "task", EntityID: "task", WorkspaceRevision: 7, CommandHash: "sha256:x"}
	approval := PlannedEvent{Type: "human_approval_attestation.recorded", EntityType: "attestation", EntityID: "att", Payload: map[string]any{"action": "task_confirm", "entityType": "task", "entityId": "task", "workspaceRevision": 7, "approvedByActorId": "human", "approvedCommandHash": "sha256:x"}}
	if evaluation := ValidateCommandAudit(expectation, []PlannedEvent{event, approval}, ActorProvenance{ExecutedBy: "agent", ApprovedBy: "human"}); !evaluation.HasErrors() {
		t.Fatal("wrong-entity domain Event accepted")
	}
}
