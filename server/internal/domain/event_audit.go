package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

type ActorProvenance struct{ InitiatedBy, ExecutedBy, ApprovedBy string }
type AuditExpectation struct {
	Command, EntityType, EntityID string
	WorkspaceRevision             int64
	CommandHash                   string
	DecisionSnapshotHash          string
	ActiveGate                    bool
}
type EventEvidenceRule struct {
	EventType           string
	RequiredPayloadKeys []string
}

var EventEvidenceRules = buildEventEvidenceRules()

func buildEventEvidenceRules() []EventEvidenceRule {
	keys := map[string][]string{
		"project.bootstrapped": {"projectId"}, "repository.registered": {"repositoryId"},
		"workspace.created": {"workspaceId"}, "workspace.activated": {"workspaceId"}, "workspace.closed": {"workspaceId"},
		"phase.created": {"phaseId"}, "phase.activated": {"phaseId"}, "phase.completed": {"phaseId"},
		"lane.created": {"laneId"}, "lane.updated": {"laneId"}, "lane.closed_out": {"laneId", "reason"}, "lane.discarded": {"laneId", "reason"},
		"task.created": {"task"}, "task.updated": {"taskId"}, "task.terminal_set": {"taskId", "reason"}, "task.terminal_cleared": {"taskId"}, "task.started": {"taskId", "clientRunId"}, "task.implemented_reported": {"taskId", "assessment", "warnings", "acknowledgedWarningCodes"}, "task.confirmed": {"taskId"}, "task.discarded": {"taskId", "reason"}, "task.rework_started": {"taskId", "reason"}, "task.blocked": {"taskId", "reason"}, "task.unblocked": {"taskId", "reason"},
		"dependency.connected": {"diff"}, "dependency.disconnected": {"diff"}, "dependency.patched": {"diff"},
		"gate.created": {"gateId", "fromPhaseId", "toPhaseId"}, "gate.task_attached": {"gateId", "taskId", "criteriaRevisionAfter"}, "gate.task_detached": {"gateId", "taskId", "criteriaRevisionAfter"}, "gate.task_passed": {"gateTaskId", "reason"}, "gate.task_pass_revoked": {"gateTaskId", "reason"}, "gate.passed": {"gateId", "conditions", "humanApprovalAttestationId", "workspaceRevision", "decisionSnapshotHash"},
		"run.started": {"runId", "taskId", "clientRunId", "kind"}, "run.succeeded": {"runId", "resultSummary"}, "run.failed": {"runId", "errorSummary"}, "run.cancelled": {"runId", "errorSummary"}, "run.interrupted": {"runId", "errorSummary"}, "run.corrected": {"runId", "previousStatus", "previousResultSummary", "previousErrorSummary", "previousEndedAt", "newStatus", "newResultSummary", "newErrorSummary", "newEndedAt", "reason"},
		"record.registered": {"recordId", "taskId", "repositoryId", "relativePath"}, "record.commit_attached": {"recordId", "commitSha", "blobSha"}, "commit.attached": {"commitId", "taskId", "repositoryId", "commitSha", "relation"}, "git.observed": {"observationId", "runId", "repositoryId", "observedAt"},
		"human_approval_attestation.recorded": {"action", "entityType", "entityId", "workspaceRevision", "approvedByActorId", "approvedCommandHash"},
	}
	rules := make([]EventEvidenceRule, 0, len(keys))
	for eventType, required := range keys {
		rules = append(rules, EventEvidenceRule{EventType: eventType, RequiredPayloadKeys: required})
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].EventType < rules[j].EventType })
	return rules
}

func ValidateEventEvidence(event PlannedEvent) Evaluation {
	evaluation := Evaluation{}
	if strings.TrimSpace(event.Type) == "" || strings.TrimSpace(event.EntityType) == "" || strings.TrimSpace(event.EntityID) == "" {
		evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: event.Type})
		return evaluation
	}
	var rule *EventEvidenceRule
	for index := range EventEvidenceRules {
		if EventEvidenceRules[index].EventType == event.Type {
			rule = &EventEvidenceRules[index]
			break
		}
	}
	if rule == nil {
		evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: event.Type, Details: map[string]any{"reason": "unknown_event_type"}})
		return evaluation
	}
	for _, key := range rule.RequiredPayloadKeys {
		value, exists := event.Payload[key]
		if !exists || !eventEvidenceValuePresent(event.Type, key, value) {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: event.Type, Details: map[string]any{"missingKey": key}})
		}
	}
	if event.Type == "gate.passed" && !nonEmptyCollection(event.Payload["conditions"]) {
		evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: event.Type, Details: map[string]any{"missingKey": "conditions"}})
	}
	if event.Type == "gate.passed" && !validGateConditionEvidence(event.Payload["conditions"]) {
		evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: event.Type, Details: map[string]any{"reason": "invalid_gate_conditions"}})
	}
	if _, hasWarnings := event.Payload["warnings"]; hasWarnings && !implementedWarningEvidenceMatches(event.Payload) {
		evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: event.Type, Details: map[string]any{"reason": "warning_acknowledgement_mismatch"}})
	}
	if key := eventEntityPayloadKey(event.Type); key != "" {
		if value, exists := event.Payload[key]; !exists || fmt.Sprint(value) != event.EntityID {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: event.Type, Details: map[string]any{"reason": "entity_payload_mismatch"}})
		}
	}
	if event.Type == "task.created" {
		if task, ok := event.Payload["task"].(Task); !ok || task.ID != event.EntityID {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: event.Type, Details: map[string]any{"reason": "entity_payload_mismatch"}})
		}
	}
	if event.Type == "human_approval_attestation.recorded" && event.Payload["action"] == "gate_pass" && !evidencePresent(event.Payload["decisionSnapshotHash"]) {
		evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: event.Type, Details: map[string]any{"missingKey": "decisionSnapshotHash"}})
	}
	evaluation.sort()
	return evaluation
}

func eventEvidenceValuePresent(eventType, key string, value any) bool {
	if eventType == "run.corrected" {
		switch key {
		case "previousResultSummary", "previousErrorSummary", "newResultSummary", "newErrorSummary":
			// Terminal Runs carry either a result or an error summary. The empty
			// counterpart is still required in correction evidence so consumers can
			// distinguish an explicitly cleared value from an omitted field.
			return value != nil
		}
	}
	return evidencePresent(value)
}

func ValidateCommandAudit(expectation AuditExpectation, events []PlannedEvent, provenance ActorProvenance) Evaluation {
	evaluation := Evaluation{}
	policy, ok := policyFor(expectation.Command, expectation.ActiveGate)
	if !ok {
		evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: expectation.Command})
		return evaluation
	}
	if strings.TrimSpace(provenance.ExecutedBy) == "" {
		evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: expectation.Command})
	}
	if provenance.InitiatedBy != "" && strings.TrimSpace(provenance.InitiatedBy) == "" {
		evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: expectation.Command})
	}
	if policy.OperationalNoEvent {
		if len(events) != 0 {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: expectation.Command})
		}
		evaluation.sort()
		return evaluation
	}
	foundDomain := false
	var domainEvent *PlannedEvent
	domainCount := 0
	var approvalEvent *PlannedEvent
	approvalCount := 0
	for index := range events {
		event := events[index]
		if event.Type == policy.EventType && event.EntityType == expectation.EntityType && event.EntityID == expectation.EntityID {
			foundDomain = true
			domainCount++
			domainEvent = &events[index]
		}
		if event.Type == "human_approval_attestation.recorded" {
			approvalCount++
			approvalEvent = &events[index]
		}
		child := ValidateEventEvidence(event)
		evaluation.Errors = append(evaluation.Errors, child.Errors...)
	}
	if !foundDomain || domainCount != 1 {
		evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: policy.EventType})
	}
	human := policy.HumanApproval == ApprovalAlways || policy.HumanApproval == ApprovalAlwaysOwner || policy.HumanApproval == ApprovalWhenFromPhaseActive && expectation.ActiveGate
	if human {
		if strings.TrimSpace(provenance.ApprovedBy) == "" || approvalEvent == nil {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeHumanApprovalRequired, EntityID: expectation.Command})
		} else if approvalCount != 1 || domainEvent == nil || !approvalMatches(*approvalEvent, expectation, provenance) || expectation.Command == "gate.pass" && !gateApprovalEvidenceMatches(*domainEvent, *approvalEvent, expectation) {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeHumanApprovalMismatch, EntityID: expectation.EntityID})
		}
	} else if approvalCount != 0 {
		evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeHumanApprovalMismatch, EntityID: expectation.EntityID})
	}
	evaluation.sort()
	return evaluation
}

func approvalMatches(event PlannedEvent, expectation AuditExpectation, provenance ActorProvenance) bool {
	if expectation.WorkspaceRevision <= 0 || strings.TrimSpace(expectation.CommandHash) == "" || event.Payload["action"] != canonicalApprovalAction(expectation.Command) || event.Payload["entityType"] != expectation.EntityType || event.Payload["entityId"] != expectation.EntityID || event.Payload["approvedByActorId"] != provenance.ApprovedBy || fmt.Sprint(event.Payload["workspaceRevision"]) != fmt.Sprint(expectation.WorkspaceRevision) || event.Payload["approvedCommandHash"] != expectation.CommandHash {
		return false
	}
	return expectation.Command != "gate.pass" || expectation.DecisionSnapshotHash != "" && event.Payload["decisionSnapshotHash"] == expectation.DecisionSnapshotHash
}

func canonicalApprovalAction(command string) string { return strings.ReplaceAll(command, ".", "_") }

func gateApprovalEvidenceMatches(domainEvent, approvalEvent PlannedEvent, expectation AuditExpectation) bool {
	return domainEvent.Payload["humanApprovalAttestationId"] == approvalEvent.EntityID && domainEvent.Payload["decisionSnapshotHash"] == expectation.DecisionSnapshotHash && fmt.Sprint(domainEvent.Payload["workspaceRevision"]) == fmt.Sprint(expectation.WorkspaceRevision+1)
}

type GatePassConditionEvidence struct {
	LinkID, TaskID string
	TaskStatus     TaskStatus
	Passed         bool
	PassReason     string
}
type GatePassEvidence struct {
	GateID                     string
	WorkspaceRevision          int64
	HumanApprovalAttestationID string
	Conditions                 []GatePassConditionEvidence
}

func ProjectGatePassEvidence(gateID string, workspaceRevision int64, attestationID string, conditions []GateTaskCondition) (GatePassEvidence, error) {
	if gateID == "" || workspaceRevision <= 0 || attestationID == "" || len(conditions) == 0 {
		return GatePassEvidence{}, &Violation{Code: CodeInvalidStateTransition}
	}
	conditionEvidence, err := projectGatePassConditions(gateID, conditions)
	if err != nil {
		return GatePassEvidence{}, err
	}
	return GatePassEvidence{GateID: gateID, WorkspaceRevision: workspaceRevision, HumanApprovalAttestationID: attestationID, Conditions: conditionEvidence}, nil
}

func projectGatePassConditions(gateID string, conditions []GateTaskCondition) ([]GatePassConditionEvidence, error) {
	result := make([]GatePassConditionEvidence, 0, len(conditions))
	seenLinks, seenTasks := map[string]bool{}, map[string]bool{}
	for _, condition := range conditions {
		if condition.GateID != gateID || condition.LinkID == "" || condition.TaskID == "" || seenLinks[condition.LinkID] || seenTasks[condition.TaskID] || condition.Passed && strings.TrimSpace(condition.PassReason) == "" || !condition.Passed && (strings.TrimSpace(condition.PassReason) != "" || condition.TaskStatus != TaskConfirmed) {
			return nil, &Violation{Code: CodeInvalidStateTransition}
		}
		seenLinks[condition.LinkID], seenTasks[condition.TaskID] = true, true
		result = append(result, GatePassConditionEvidence{LinkID: condition.LinkID, TaskID: condition.TaskID, TaskStatus: condition.TaskStatus, Passed: condition.Passed, PassReason: strings.TrimSpace(condition.PassReason)})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].LinkID < result[j].LinkID })
	return result, nil
}

func GateDecisionSnapshotHash(workspace Workspace, gate Gate, conditions []GateTaskCondition) string {
	ordered := append([]GateTaskCondition(nil), conditions...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].LinkID < ordered[j].LinkID })
	rows := make([]any, 0, len(ordered))
	for _, condition := range ordered {
		rows = append(rows, []any{condition.LinkID, condition.TaskID, condition.TaskStatus, condition.Passed, strings.TrimSpace(condition.PassReason)})
	}
	payload, _ := json.Marshal([]any{gate.ID, gate.CriteriaRevision, gate.FromPhaseID, gate.ToPhaseID, rows, workspace.Revision})
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func MutationCommandHash(plan DomainMutationPlan, workspaceRevision int64, commandInput any) string {
	payload, _ := json.Marshal(struct {
		Command              string
		WorkspaceRevision    int64
		RequiredCapability   string
		HumanApproval        HumanApprovalMode
		DecisionSnapshotHash string
		CommandInput         any
		ProjectedDiff        any
		Warnings             []Diagnostic
	}{
		Command: plan.Command, WorkspaceRevision: workspaceRevision, RequiredCapability: plan.RequiredCapability,
		HumanApproval: plan.HumanApproval, DecisionSnapshotHash: plan.DecisionSnapshotHash,
		CommandInput: commandInput, ProjectedDiff: plan.ProjectedDiff, Warnings: plan.Evaluation.Warnings,
	})
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func eventEntityPayloadKey(eventType string) string {
	keys := map[string]string{
		"project.bootstrapped": "projectId", "repository.registered": "repositoryId",
		"workspace.created": "workspaceId", "workspace.activated": "workspaceId", "workspace.closed": "workspaceId",
		"phase.created": "phaseId", "phase.activated": "phaseId", "phase.completed": "phaseId",
		"lane.created": "laneId", "lane.updated": "laneId", "lane.closed_out": "laneId", "lane.discarded": "laneId",
		"task.updated": "taskId", "task.terminal_set": "taskId", "task.terminal_cleared": "taskId", "task.started": "taskId", "task.implemented_reported": "taskId", "task.confirmed": "taskId", "task.discarded": "taskId", "task.rework_started": "taskId", "task.blocked": "taskId", "task.unblocked": "taskId",
		"gate.created": "gateId", "gate.task_attached": "gateId", "gate.task_detached": "gateId", "gate.task_passed": "gateTaskId", "gate.task_pass_revoked": "gateTaskId", "gate.passed": "gateId",
		"run.started": "runId", "run.succeeded": "runId", "run.failed": "runId", "run.cancelled": "runId", "run.interrupted": "runId", "run.corrected": "runId",
		"record.registered": "recordId", "record.commit_attached": "recordId", "commit.attached": "commitId", "git.observed": "observationId",
	}
	return keys[eventType]
}

func validGateConditionEvidence(value any) bool {
	conditions, ok := value.([]GatePassConditionEvidence)
	if !ok || len(conditions) == 0 {
		return false
	}
	seenLinks, seenTasks := map[string]bool{}, map[string]bool{}
	for _, condition := range conditions {
		if condition.LinkID == "" || condition.TaskID == "" || seenLinks[condition.LinkID] || seenTasks[condition.TaskID] || condition.Passed && strings.TrimSpace(condition.PassReason) == "" || !condition.Passed && condition.TaskStatus != TaskConfirmed {
			return false
		}
		seenLinks[condition.LinkID], seenTasks[condition.TaskID] = true, true
	}
	return true
}

func evidencePresent(value any) bool {
	if value == nil {
		return false
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.String:
		return strings.TrimSpace(reflected.String()) != ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return reflected.Int() > 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return reflected.Uint() > 0
	case reflect.Pointer, reflect.Interface, reflect.Slice, reflect.Map:
		return !reflected.IsNil()
	}
	return true
}

func nonEmptyCollection(value any) bool {
	if value == nil {
		return false
	}
	reflected := reflect.ValueOf(value)
	return (reflected.Kind() == reflect.Array || reflected.Kind() == reflect.Slice || reflected.Kind() == reflect.Map) && reflected.Len() > 0
}

func implementedWarningEvidenceMatches(payload map[string]any) bool {
	warnings, warningsOK := stringCollection(payload["warnings"])
	acknowledged, acknowledgedOK := stringCollection(payload["acknowledgedWarningCodes"])
	if !warningsOK || !acknowledgedOK || len(warnings) != len(acknowledged) {
		return false
	}
	sort.Strings(warnings)
	sort.Strings(acknowledged)
	for index := range warnings {
		if warnings[index] == "" || warnings[index] != acknowledged[index] || index > 0 && warnings[index] == warnings[index-1] {
			return false
		}
	}
	return true
}

func stringCollection(value any) ([]string, bool) {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...), typed != nil
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, false
			}
			result = append(result, text)
		}
		return result, typed != nil
	default:
		return nil, false
	}
}
