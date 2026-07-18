package collab

import (
	"sort"
	"strings"
	"time"

	"github.com/jazzcake/baley/server/internal/authz"
	"github.com/jazzcake/baley/server/internal/domain"
)

type AuditEvent struct {
	ID                   string
	WorkspaceID          string
	LaneIDs              []string
	TaskScopes           []AuditTaskScope
	Command              string
	CommandID            string
	CommandHash          string
	MutationEntityType   string
	MutationEntityID     string
	DecisionSnapshotHash string
	EventType            string
	EntityType           string
	EntityID             string
	WorkspaceRevision    int64
	InitiatedBy          string
	InitiatedKind        authz.ActorKind
	ExecutedBy           string
	ExecutedKind         authz.ActorKind
	ApprovedBy           string
	ApprovedKind         authz.ActorKind
	ApprovalEvidence     *AuditApprovalEvidence
	ActiveGate           bool
	OccurredAt           time.Time
}

type AuditTaskScope struct {
	TaskID string
	LaneID string
}

type AuditApprovalEvidence struct {
	Attestation       domain.HumanApprovalAttestation
	WorkspaceID       string
	ExecutedCommandID string
}

type AuditQuery struct {
	WorkspaceID   string
	LaneID        string
	TaskID        string
	ImportantOnly bool
}

type ActorDisplay struct {
	ActorID string          `json:"actorId,omitempty"`
	Kind    authz.ActorKind `json:"kind,omitempty"`
	Missing bool            `json:"missing"`
}

type AuditEntry struct {
	ID                string
	EventType         string
	EntityType        string
	EntityID          string
	WorkspaceRevision int64
	OccurredAt        time.Time
	InitiatedBy       ActorDisplay
	ExecutedBy        ActorDisplay
	ApprovedBy        ActorDisplay
	Important         bool
	AgentExecuted     bool
	ApprovalRequired  bool
	ApprovalRecorded  bool
}

func BuildAuditTimeline(query AuditQuery, events []AuditEvent) ([]AuditEntry, error) {
	if !cleanIdentifier(query.WorkspaceID) || query.LaneID != "" && !cleanIdentifier(query.LaneID) || query.TaskID != "" && !cleanIdentifier(query.TaskID) {
		return nil, ErrInvalidAuditInput
	}
	result := []AuditEntry{}
	seen := map[string]bool{}
	attestationCommands, commandAttestations := map[string]string{}, map[string]string{}
	type approvalBinding struct {
		present                                                                    bool
		workspaceID, executedCommandID, id, action, entityType, entityID           string
		approvedByActorID, approverRole, approvedCommandHash, decisionSnapshotHash string
		workspaceRevision                                                          int64
	}
	type commandBinding struct {
		workspaceID, command, commandHash, entityType, entityID string
		decisionSnapshotHash                                    string
		initiatedBy, executedBy, approvedBy                     string
		initiatedKind, executedKind, approvedKind               authz.ActorKind
		revision                                                int64
		activeGate                                              bool
		approval                                                approvalBinding
	}
	commandBindings := map[string]commandBinding{}
	for _, event := range events {
		if !validAuditEvent(event) || seen[event.ID] {
			return nil, ErrInvalidAuditInput
		}
		seen[event.ID] = true
		approval := approvalBinding{}
		if event.ApprovalEvidence != nil {
			attestation := event.ApprovalEvidence.Attestation
			approval = approvalBinding{true, event.ApprovalEvidence.WorkspaceID, event.ApprovalEvidence.ExecutedCommandID, attestation.ID, attestation.Action, attestation.EntityType, attestation.EntityID, attestation.ApprovedByActorID, attestation.ApproverRole, attestation.ApprovedCommandHash, attestation.DecisionSnapshotHash, attestation.WorkspaceRevision}
		}
		binding := commandBinding{event.WorkspaceID, event.Command, event.CommandHash, event.MutationEntityType, event.MutationEntityID, event.DecisionSnapshotHash, event.InitiatedBy, event.ExecutedBy, event.ApprovedBy, event.InitiatedKind, event.ExecutedKind, event.ApprovedKind, event.WorkspaceRevision, event.ActiveGate, approval}
		if existing, exists := commandBindings[event.CommandID]; exists && existing != binding {
			return nil, ErrInvalidAuditInput
		}
		commandBindings[event.CommandID] = binding
		if event.ApprovalEvidence != nil {
			attestationID := event.ApprovalEvidence.Attestation.ID
			if cleanIdentifier(attestationID) && attestationCommands[attestationID] != "" && attestationCommands[attestationID] != event.CommandID {
				return nil, ErrInvalidAuditInput
			}
			if cleanIdentifier(attestationID) && commandAttestations[event.CommandID] != "" && commandAttestations[event.CommandID] != attestationID {
				return nil, ErrInvalidAuditInput
			}
			if cleanIdentifier(attestationID) {
				attestationCommands[attestationID], commandAttestations[event.CommandID] = event.CommandID, attestationID
			}
		}
		if event.WorkspaceID != query.WorkspaceID || query.LaneID != "" && !eventContainsLane(event, query.LaneID) || query.TaskID != "" && !eventContainsTask(event, query.TaskID) {
			continue
		}
		important := auditEventImportance[event.EventType]
		if query.ImportantOnly && !important {
			continue
		}
		approvalRequired, _ := approvalRequired(event.Command, event.ActiveGate)
		entry := AuditEntry{
			ID:                event.ID,
			EventType:         event.EventType,
			EntityType:        event.EntityType,
			EntityID:          event.EntityID,
			WorkspaceRevision: event.WorkspaceRevision,
			OccurredAt:        event.OccurredAt,
			InitiatedBy:       actorDisplay(event.InitiatedBy, event.InitiatedKind),
			ExecutedBy:        actorDisplay(event.ExecutedBy, event.ExecutedKind),
			ApprovedBy:        actorDisplay(event.ApprovedBy, event.ApprovedKind),
			Important:         important,
			AgentExecuted:     event.ExecutedKind == authz.ActorAgent,
			ApprovalRequired:  approvalRequired,
			ApprovalRecorded:  approvalRequired && approvalEvidenceMatches(event),
		}
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool {
		if !result[i].OccurredAt.Equal(result[j].OccurredAt) {
			return result[i].OccurredAt.After(result[j].OccurredAt)
		}
		if result[i].WorkspaceRevision != result[j].WorkspaceRevision {
			return result[i].WorkspaceRevision > result[j].WorkspaceRevision
		}
		return result[i].ID < result[j].ID
	})
	return result, nil
}

func validAuditEvent(event AuditEvent) bool {
	if !cleanIdentifier(event.ID) || !cleanIdentifier(event.WorkspaceID) || !cleanIdentifier(event.CommandID) || !cleanIdentifier(event.EntityType) || !cleanIdentifier(event.EntityID) || !cleanIdentifier(event.MutationEntityType) || !cleanIdentifier(event.MutationEntityID) || event.WorkspaceRevision <= 0 || event.OccurredAt.IsZero() || !knownEvent(event.EventType) || !validScopeIDs(event.LaneIDs) || !validTaskScopes(event.TaskScopes) {
		return false
	}
	if event.EntityType == "lane" && !eventContainsLane(event, event.EntityID) || event.EntityType == "task" && !eventContainsTask(event, event.EntityID) {
		return false
	}
	if event.MutationEntityType == "lane" && !eventContainsLane(event, event.MutationEntityID) || event.MutationEntityType == "task" && !eventContainsTask(event, event.MutationEntityID) {
		return false
	}
	if eventRequiresTaskScope(event.EventType) && len(event.TaskScopes) == 0 {
		return false
	}
	primary, allowed := auditCommandEventRelation(event.Command, event.EventType, event.ActiveGate)
	if !allowed || primary && (event.EntityType != event.MutationEntityType || event.EntityID != event.MutationEntityID) {
		return false
	}
	return validActorReference(event.InitiatedBy, event.InitiatedKind, false) && validActorReference(event.ExecutedBy, event.ExecutedKind, true) && validActorReference(event.ApprovedBy, event.ApprovedKind, false)
}

func validScopeIDs(values []string) bool {
	seen := map[string]bool{}
	for _, value := range values {
		if !cleanIdentifier(value) || seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}

func validTaskScopes(values []AuditTaskScope) bool {
	seen := map[string]bool{}
	for _, value := range values {
		if !cleanIdentifier(value.TaskID) || !cleanIdentifier(value.LaneID) || seen[value.TaskID] {
			return false
		}
		seen[value.TaskID] = true
	}
	return true
}

func eventContainsTask(event AuditEvent, taskID string) bool {
	for _, scope := range event.TaskScopes {
		if scope.TaskID == taskID {
			return true
		}
	}
	return false
}

func eventContainsLane(event AuditEvent, laneID string) bool {
	if containsID(event.LaneIDs, laneID) {
		return true
	}
	for _, scope := range event.TaskScopes {
		if scope.LaneID == laneID {
			return true
		}
	}
	return false
}

func containsID(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func validActorReference(id string, kind authz.ActorKind, required bool) bool {
	if id == "" {
		return !required && kind == ""
	}
	return cleanIdentifier(id) && (kind == authz.ActorHuman || kind == authz.ActorAgent || kind == authz.ActorSystem)
}

func actorDisplay(id string, kind authz.ActorKind) ActorDisplay {
	id = strings.TrimSpace(id)
	return ActorDisplay{ActorID: id, Kind: kind, Missing: id == ""}
}

func knownEvent(value string) bool {
	for _, rule := range domain.EventEvidenceRules {
		if rule.EventType == value {
			return true
		}
	}
	return false
}

func approvalRequired(command string, active bool) (bool, bool) {
	for _, policy := range domain.MutationPolicies {
		if policy.Name == command {
			required := policy.HumanApproval == domain.ApprovalAlways || policy.HumanApproval == domain.ApprovalAlwaysOwner || policy.HumanApproval == domain.ApprovalWhenFromPhaseActive && active
			return required, true
		}
	}
	return false, false
}

func auditCommandAllowsEvent(command, eventType string, activeGate bool) bool {
	_, allowed := auditCommandEventRelation(command, eventType, activeGate)
	return allowed
}

func auditCommandEventRelation(command, eventType string, activeGate bool) (bool, bool) {
	for _, policy := range domain.MutationPolicies {
		if policy.Name != command {
			continue
		}
		if policy.EventType == eventType {
			return true, true
		}
		if eventType == "human_approval_attestation.recorded" {
			required, _ := approvalRequired(command, activeGate)
			return false, required
		}
		for _, secondary := range secondaryAuditEvents[command] {
			if secondary == eventType {
				return false, true
			}
		}
		return false, false
	}
	return false, false
}

var secondaryAuditEvents = map[string][]string{
	"project.bootstrap":  {"workspace.created", "repository.registered"},
	"workspace.activate": {"phase.activated"},
	"workspace.close":    {"phase.completed"},
	"gate.pass":          {"phase.completed", "phase.activated"},
	"run.start":          {"task.started"},
}

func eventRequiresTaskScope(eventType string) bool {
	return strings.HasPrefix(eventType, "task.") || strings.HasPrefix(eventType, "dependency.") || strings.HasPrefix(eventType, "run.") || strings.HasPrefix(eventType, "record.") || eventType == "commit.attached" || strings.HasPrefix(eventType, "gate.task_")
}

func approvalEvidenceMatches(event AuditEvent) bool {
	evidence := event.ApprovalEvidence
	if evidence == nil || event.ApprovedKind != authz.ActorHuman || !cleanIdentifier(event.ApprovedBy) || !cleanIdentifier(event.CommandHash) || !cleanIdentifier(evidence.Attestation.ID) || !cleanIdentifier(evidence.WorkspaceID) || !cleanIdentifier(evidence.ExecutedCommandID) || event.WorkspaceRevision <= 1 {
		return false
	}
	attestation := evidence.Attestation
	if evidence.WorkspaceID != event.WorkspaceID || evidence.ExecutedCommandID != event.CommandID || attestation.Action != strings.ReplaceAll(event.Command, ".", "_") || attestation.EntityType != event.MutationEntityType || attestation.EntityID != event.MutationEntityID || attestation.WorkspaceRevision != event.WorkspaceRevision-1 || attestation.ApprovedByActorID != event.ApprovedBy || attestation.ApprovedCommandHash != event.CommandHash {
		return false
	}
	if event.EventType == "human_approval_attestation.recorded" && attestation.ID != event.EntityID {
		return false
	}
	if event.Command == "workspace.close" && attestation.ApproverRole != "owner" {
		return false
	}
	return event.Command != "gate.pass" || cleanIdentifier(event.DecisionSnapshotHash) && attestation.DecisionSnapshotHash == event.DecisionSnapshotHash
}

// Every domain Event type is deliberately classified. The companion totality
// test makes a newly added Event fail until its audit importance is decided.
var auditEventImportance = map[string]bool{
	"project.bootstrapped":                false,
	"repository.registered":               false,
	"workspace.created":                   true,
	"workspace.activated":                 true,
	"workspace.closed":                    true,
	"phase.created":                       false,
	"phase.activated":                     true,
	"phase.completed":                     true,
	"lane.created":                        false,
	"lane.updated":                        false,
	"lane.closed_out":                     true,
	"lane.discarded":                      true,
	"gate.created":                        false,
	"task.created":                        false,
	"task.updated":                        false,
	"task.terminal_set":                   true,
	"task.terminal_cleared":               true,
	"task.started":                        true,
	"task.implemented_reported":           true,
	"task.confirmed":                      true,
	"task.discarded":                      true,
	"task.rework_started":                 true,
	"task.blocked":                        true,
	"task.unblocked":                      true,
	"dependency.connected":                true,
	"dependency.disconnected":             true,
	"dependency.patched":                  true,
	"gate.task_attached":                  true,
	"gate.task_detached":                  true,
	"gate.task_passed":                    true,
	"gate.task_pass_revoked":              true,
	"gate.passed":                         true,
	"run.started":                         true,
	"run.succeeded":                       true,
	"run.failed":                          true,
	"run.cancelled":                       true,
	"run.interrupted":                     true,
	"run.corrected":                       true,
	"record.registered":                   false,
	"record.commit_attached":              false,
	"commit.attached":                     false,
	"git.observed":                        false,
	"human_approval_attestation.recorded": false,
}

const ErrInvalidAuditInput conflictError = "invalid audit input"
