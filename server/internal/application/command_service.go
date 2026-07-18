package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jazzcake/baley/server/internal/domain"
)

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service { return &Service{repo: repo, now: time.Now} }

type taskConfirmArgs struct {
	WorkspaceID string `json:"workspaceId"`
	TaskID      int    `json:"taskId"`
}
type gateTaskArgs struct {
	WorkspaceID string `json:"workspaceId"`
	GateTaskID  string `json:"gateTaskId"`
	Reason      string `json:"reason"`
}
type gatePassArgs struct {
	WorkspaceID string `json:"workspaceId"`
	GateID      string `json:"gateId"`
}

func (s *Service) Preview(ctx context.Context, request CommandRequest) (PreviewResult, error) {
	wid, typed, err := decodeArguments(request.Name, request.Arguments)
	if err != nil {
		return PreviewResult{}, err
	}
	snapshot, err := s.repo.LoadSnapshot(ctx, wid)
	if err != nil {
		return PreviewResult{}, err
	}
	if request.Envelope.ExpectedWorkspaceRevision == 0 {
		request.Envelope.ExpectedWorkspaceRevision = snapshot.Workspace.Revision
	}
	result, _, err := s.evaluate(ctx, request, typed, snapshot, false)
	return result, err
}

func (s *Service) Execute(ctx context.Context, request CommandRequest) (ExecutionResult, error) {
	wid, typed, err := decodeArguments(request.Name, request.Arguments)
	if err != nil {
		return ExecutionResult{}, err
	}
	if strings.TrimSpace(request.Envelope.IdempotencyKey) == "" || strings.TrimSpace(request.Envelope.ExecutedByActorID) == "" || request.Envelope.ExpectedWorkspaceRevision == 0 {
		return ExecutionResult{}, &CommandError{Code: "invalid_request", Message: "execute requires idempotencyKey, executedByActorId and expectedWorkspaceRevision"}
	}
	// Hash is finalized in the locked callback because Gate commands bind the current condition snapshot.
	fingerprint := hashRequestFingerprint(request.Name, typed, request.Envelope.ExpectedWorkspaceRevision)
	return s.repo.Execute(ctx, wid, request, fingerprint, func(snapshot Snapshot) (PreviewResult, MutationPlan, error) {
		return s.evaluate(ctx, request, typed, snapshot, true)
	})
}

func (s *Service) evaluate(ctx context.Context, request CommandRequest, typed any, snapshot Snapshot, executing bool) (PreviewResult, MutationPlan, error) {
	result := PreviewResult{ExpectedWorkspaceRevision: request.Envelope.ExpectedWorkspaceRevision, Errors: []Diagnostic{}, Warnings: []Diagnostic{}, Advisories: []Diagnostic{}}
	plan := MutationPlan{CommandName: request.Name, Action: request.Name}
	if request.Envelope.ExpectedWorkspaceRevision != snapshot.Workspace.Revision {
		result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeStaleRevision, EntityID: snapshot.Workspace.ID})
		if executing {
			return result, plan, &CommandError{Code: domain.CodeStaleRevision, Message: "command evaluation failed: " + domain.CodeStaleRevision}
		}
	}
	var decisionHash string
	switch args := typed.(type) {
	case taskConfirmArgs:
		result.RequiredCapability = "task:approve"
		plan.EntityType = "task"
		plan.EntityID = fmt.Sprint(args.TaskID)
		task := findTaskByPublicID(snapshot.Tasks, args.TaskID)
		if task == nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: fmt.Sprint(args.TaskID)})
		} else if task.Status != "implemented" {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: task.ID})
		} else {
			plan.TaskID = task.ID
			plan.TaskStatus = "confirmed"
			result.ProjectedDiff = map[string]any{"taskId": args.TaskID, "status": map[string]string{"before": task.Status, "after": "confirmed"}}
			plan.Events = []EventWrite{{Type: "task.confirmed", Payload: map[string]any{"taskId": args.TaskID}}}
		}
	case gateTaskArgs:
		result.RequiredCapability = "gate:approve"
		plan.EntityType = "gate_task"
		plan.EntityID = args.GateTaskID
		gate, condition := findGateTask(snapshot.Gates, args.GateTaskID)
		if gate == nil || condition == nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: args.GateTaskID})
			break
		}
		if snapshot.Workspace.ActivePhaseID == nil || *snapshot.Workspace.ActivePhaseID != gate.FromPhaseID {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeGateNotCurrent, EntityID: gate.ID})
			break
		}
		decisionHash = hashDecision(snapshot, *gate)
		if gate.PassedAt != nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: gate.ID})
			break
		}
		if strings.TrimSpace(args.Reason) == "" {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: condition.ID})
			break
		}
		isPass := request.Name == "gate.pass_task"
		if isPass == (condition.PassedAt != nil) {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: condition.ID})
			break
		}
		plan.GateTaskID = condition.ID
		plan.GateTaskPassed = isPass
		plan.GateTaskReason = strings.TrimSpace(args.Reason)
		plan.GateID = gate.ID
		after := "passed"
		event := "gate.task_passed"
		if !isPass {
			after = "required"
			event = "gate.task_pass_revoked"
		}
		result.ProjectedDiff = map[string]any{"gateTaskId": condition.ID, "condition": map[string]string{"after": after}}
		plan.Events = []EventWrite{{Type: event, Payload: map[string]any{"gateTaskId": condition.ID, "reason": plan.GateTaskReason}}}
	case gatePassArgs:
		result.RequiredCapability = "gate:approve"
		plan.EntityType = "gate"
		plan.EntityID = args.GateID
		gate := findGate(snapshot.Gates, args.GateID)
		if gate == nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: args.GateID})
			break
		}
		decisionHash = hashDecision(snapshot, *gate)
		conditions := make([]domain.GateTaskCondition, 0, len(gate.Conditions))
		for _, c := range gate.Conditions {
			task := findTask(snapshot.Tasks, c.TaskID)
			status := domain.TaskPending
			if task != nil {
				status = domain.TaskStatus(task.Status)
			}
			conditions = append(conditions, domain.GateTaskCondition{LinkID: c.ID, TaskID: c.TaskID, TaskStatus: status, Passed: c.PassedAt != nil})
		}
		from := findPhase(snapshot.Phases, gate.FromPhaseID)
		to := findPhase(snapshot.Phases, gate.ToPhaseID)
		if from == nil || to == nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: gate.ID})
			break
		}
		_, transitionErr := domain.PlanGatePass(domain.Gate{ID: gate.ID, FromPhaseID: gate.FromPhaseID, ToPhaseID: gate.ToPhaseID, PassedAt: gate.PassedAt}, domain.Phase{ID: from.ID, Position: from.Position, State: domain.PhaseState(from.State)}, domain.Phase{ID: to.ID, Position: to.Position, State: domain.PhaseState(to.State)}, conditions, s.now())
		if transitionErr != nil {
			result.Errors = append(result.Errors, Diagnostic{Code: transitionErr.(*domain.Violation).Code, EntityID: gate.ID})
			break
		}
		plan.GateID = gate.ID
		plan.FromPhaseID = from.ID
		plan.ToPhaseID = to.ID
		evidence := make([]map[string]any, 0, len(gate.Conditions))
		for _, c := range gate.Conditions {
			task := findTask(snapshot.Tasks, c.TaskID)
			status := ""
			if task != nil {
				status = task.Status
			}
			evidence = append(evidence, map[string]any{"linkId": c.ID, "taskId": c.TaskID, "taskStatus": status, "passed": c.PassedAt != nil, "passReasonPresent": c.PassReason != nil})
		}
		plan.Events = []EventWrite{{Type: "gate.passed", Payload: map[string]any{"gateId": gate.ID, "conditions": evidence}}, {Type: "phase.completed", Payload: map[string]any{"phaseId": from.ID}}, {Type: "phase.activated", Payload: map[string]any{"phaseId": to.ID}}}
		result.ProjectedDiff = map[string]any{"gate": map[string]string{"id": gate.ID, "after": "passed"}, "phases": []map[string]string{{"id": from.ID, "after": "completed"}, {"id": to.ID, "after": "active"}}}
	default:
		return result, plan, &CommandError{Code: "invalid_request", Message: "unsupported command"}
	}
	result.DecisionSnapshotHash = decisionHash
	result.CommandHash = hashCommand(request.Name, typed, request.Envelope.ExpectedWorkspaceRevision, decisionHash)
	if !executing || request.Envelope.HumanApprovalAttestation == nil {
		result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeHumanApprovalRequired, EntityID: plan.EntityID})
	}
	if executing {
		att := request.Envelope.HumanApprovalAttestation
		valid := att != nil && att.ApprovedCommandHash == result.CommandHash && (decisionHash == "" || att.DecisionSnapshotHash == decisionHash)
		if valid {
			valid = snapshot.HumanActorIDs[att.ApprovedByActorID]
		}
		if !valid {
			result.Errors = removeDiagnostic(result.Errors, domain.CodeHumanApprovalRequired)
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeHumanApprovalMismatch, EntityID: plan.EntityID})
		}
	}
	sort.Slice(result.Errors, func(i, j int) bool { return result.Errors[i].Code < result.Errors[j].Code })
	if executing && len(result.Errors) > 0 {
		return result, plan, &CommandError{Code: result.Errors[0].Code, Message: "command evaluation failed: " + result.Errors[0].Code}
	}
	return result, plan, nil
}

func decodeArguments(name string, raw json.RawMessage) (string, any, error) {
	var target any
	switch name {
	case "task.confirm":
		target = &taskConfirmArgs{}
	case "gate.pass_task", "gate.revoke_task_pass":
		target = &gateTaskArgs{}
	case "gate.pass":
		target = &gatePassArgs{}
	default:
		return "", nil, &CommandError{Code: "invalid_request", Message: "unsupported command: " + name}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return "", nil, &CommandError{Code: "invalid_request", Message: err.Error()}
	}
	switch v := target.(type) {
	case *taskConfirmArgs:
		return v.WorkspaceID, *v, nil
	case *gateTaskArgs:
		return v.WorkspaceID, *v, nil
	case *gatePassArgs:
		return v.WorkspaceID, *v, nil
	}
	panic("unreachable")
}

func hashCommand(name string, args any, revision int64, snapshot string) string {
	payload, _ := json.Marshal(struct {
		ContractVersion string `json:"contractVersion"`
		Name            string `json:"commandName"`
		Arguments       any    `json:"arguments"`
		Revision        int64  `json:"expectedWorkspaceRevision"`
		Snapshot        string `json:"decisionSnapshotHash,omitempty"`
	}{"1.0.0", name, args, revision, snapshot})
	return sha(payload)
}
func hashRequestFingerprint(name string, args any, revision int64) string {
	payload, _ := json.Marshal(struct {
		ContractVersion string `json:"contractVersion"`
		Name            string `json:"commandName"`
		Arguments       any    `json:"arguments"`
		Revision        int64  `json:"expectedWorkspaceRevision"`
	}{"1.0.0", name, args, revision})
	return sha(payload)
}
func hashDecision(snapshot Snapshot, gate GateProjection) string {
	sort.Slice(gate.Conditions, func(i, j int) bool { return gate.Conditions[i].ID < gate.Conditions[j].ID })
	rows := make([]any, 0, len(gate.Conditions))
	for _, c := range gate.Conditions {
		t := findTask(snapshot.Tasks, c.TaskID)
		status := ""
		if t != nil {
			status = t.Status
		}
		rows = append(rows, []any{c.ID, c.TaskID, status, c.PassedAt != nil, c.PassReason != nil})
	}
	payload, _ := json.Marshal([]any{gate.ID, gate.CriteriaRevision, gate.FromPhaseID, gate.ToPhaseID, rows, snapshot.Workspace.Revision})
	return sha(payload)
}

func DecisionSnapshotHash(snapshot Snapshot, gate GateProjection) string {
	return hashDecision(snapshot, gate)
}
func sha(value []byte) string {
	sum := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func removeDiagnostic(values []Diagnostic, code string) []Diagnostic {
	out := values[:0]
	for _, v := range values {
		if v.Code != code {
			out = append(out, v)
		}
	}
	return out
}
func findTaskByPublicID(v []TaskProjection, id int) *TaskProjection {
	for i := range v {
		if v[i].PublicID == id {
			return &v[i]
		}
	}
	return nil
}
func findTask(v []TaskProjection, id string) *TaskProjection {
	for i := range v {
		if v[i].ID == id {
			return &v[i]
		}
	}
	return nil
}
func findGate(v []GateProjection, id string) *GateProjection {
	for i := range v {
		if v[i].ID == id {
			return &v[i]
		}
	}
	return nil
}
func findGateTask(v []GateProjection, id string) (*GateProjection, *GateTaskProjection) {
	for i := range v {
		for j := range v[i].Conditions {
			if v[i].Conditions[j].ID == id {
				return &v[i], &v[i].Conditions[j]
			}
		}
	}
	return nil, nil
}
func findPhase(v []PhaseProjection, id string) *PhaseProjection {
	for i := range v {
		if v[i].ID == id {
			return &v[i]
		}
	}
	return nil
}
