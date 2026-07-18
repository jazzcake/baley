package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/domain"
)

type InvocationKind string

const (
	QueryInvocation    InvocationKind = "query"
	MutationInvocation InvocationKind = "mutation"
)

type Invocation struct {
	Kind                      InvocationKind
	Name                      string
	WorkspaceID               string
	Arguments                 json.RawMessage
	Execute                   bool
	IdempotencyKey            string
	ExecutedByActorID         string
	ExpectedWorkspaceRevision int64
	AcknowledgedWarningCodes  []string
}

type QueryRequest struct {
	Name        string          `json:"name"`
	WorkspaceID string          `json:"workspaceId"`
	Arguments   json.RawMessage `json:"arguments"`
}

type Client interface {
	Query(context.Context, QueryRequest) (json.RawMessage, error)
	Preview(context.Context, application.CommandRequest) (application.PreviewResult, error)
	Execute(context.Context, application.CommandRequest) (application.ExecutionResult, error)
}

type Approval struct {
	ApprovedByActorID         string
	ApprovedCommandHash       string
	ExpectedWorkspaceRevision int64
	DecisionSnapshotHash      string
	StatementHash             string
	ConversationRef           string
}

type Outcome struct {
	QueryResult                    json.RawMessage
	Preview                        *application.PreviewResult
	Execution                      *application.ExecutionResult
	ApprovalRequired               bool
	WarningAcknowledgementRequired []string
}

type StructuredError struct {
	Code    string
	Message string
}

func (e *StructuredError) Error() string { return e.Message }

var queryNames = map[string]bool{
	"workspace.get": true, "workspace.graph": true, "lane.brief": true,
	"task.get": true, "task.list": true, "task.context": true,
	"gate.status": true, "run.list": true, "record.list": true,
	"event.list": true, "decision.list": true,
}

var primaryArgument = map[string]string{
	"lane.brief": "laneId",
	"task.get":   "taskId", "task.context": "taskId",
	"gate.status": "gateId",
	"lane.update": "laneId", "lane.close_out": "laneId", "lane.discard": "laneId",
	"task.update": "taskId", "task.set_terminal": "taskId", "task.clear_terminal": "taskId", "task.block": "taskId", "task.unblock": "taskId", "task.report_implemented": "taskId", "task.confirm": "taskId", "task.discard": "taskId", "task.rework": "taskId",
	"gate.attach_task": "gateId", "gate.detach_task": "gateId", "gate.pass": "gateId",
	"gate.pass_task": "gateTaskId", "gate.revoke_task_pass": "gateTaskId",
	"run.start": "taskId", "run.heartbeat": "runId", "run.succeed": "runId", "run.fail": "runId", "run.cancel": "runId", "run.interrupt": "runId", "run.correct": "runId",
	"record.register": "recordId", "record.attach_commit": "recordId",
}

func Parse(args []string) (Invocation, error) {
	if len(args) < 2 || strings.HasPrefix(args[0], "-") || strings.HasPrefix(args[1], "-") {
		return Invocation{}, invalid("expected <resource> <action>")
	}
	name := strings.ToLower(args[0]) + "." + strings.ReplaceAll(strings.ToLower(args[1]), "-", "_")
	kind := QueryInvocation
	if !queryNames[name] {
		kind = MutationInvocation
		found := false
		for _, policy := range domain.MutationPolicies {
			if policy.Name == name {
				found = true
				break
			}
		}
		if !found {
			return Invocation{}, invalid("unsupported command: " + name)
		}
	}
	invocation := Invocation{Kind: kind, Name: name}
	arguments := map[string]any{}
	positionals := []string{}
	for index := 2; index < len(args); index++ {
		value := args[index]
		if !strings.HasPrefix(value, "--") {
			positionals = append(positionals, value)
			continue
		}
		flag := strings.TrimPrefix(value, "--")
		if flag == "execute" {
			invocation.Execute = true
			continue
		}
		if index+1 >= len(args) || strings.HasPrefix(args[index+1], "--") {
			return Invocation{}, invalid("missing value for --" + flag)
		}
		index++
		flagValue := args[index]
		switch flag {
		case "workspace":
			invocation.WorkspaceID = strings.TrimSpace(flagValue)
		case "actor":
			invocation.ExecutedByActorID = strings.TrimSpace(flagValue)
		case "idempotency":
			invocation.IdempotencyKey = strings.TrimSpace(flagValue)
		case "revision":
			revision, err := strconv.ParseInt(flagValue, 10, 64)
			if err != nil || revision < 0 {
				return Invocation{}, invalid("invalid --revision")
			}
			invocation.ExpectedWorkspaceRevision = revision
		case "ack":
			if strings.TrimSpace(flagValue) == "" {
				return Invocation{}, invalid("blank --ack")
			}
			invocation.AcknowledgedWarningCodes = append(invocation.AcknowledgedWarningCodes, strings.TrimSpace(flagValue))
		case "arg":
			parts := strings.SplitN(flagValue, "=", 2)
			if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
				return Invocation{}, invalid("--arg requires key=value")
			}
			if _, duplicate := arguments[parts[0]]; duplicate {
				return Invocation{}, invalid("duplicate argument: " + parts[0])
			}
			arguments[parts[0]] = parseValue(parts[1])
		default:
			key := argumentKey(flag)
			if _, duplicate := arguments[key]; duplicate {
				return Invocation{}, invalid("duplicate argument: " + key)
			}
			arguments[key] = parseValue(flagValue)
		}
	}
	if invocation.WorkspaceID == "" {
		return Invocation{}, invalid("--workspace is required")
	}
	if _, duplicate := arguments["workspaceId"]; duplicate {
		return Invocation{}, invalid("workspace supplied twice")
	}
	arguments["workspaceId"] = invocation.WorkspaceID
	if key := primaryArgument[name]; key != "" {
		if len(positionals) != 1 {
			return Invocation{}, invalid(name + " requires one target")
		}
		if _, duplicate := arguments[key]; duplicate {
			return Invocation{}, invalid("target supplied twice: " + key)
		}
		arguments[key] = parseValue(positionals[0])
	} else if len(positionals) != 0 {
		return Invocation{}, invalid(name + " does not accept a positional target")
	}
	if kind == MutationInvocation && (invocation.IdempotencyKey == "" || invocation.ExecutedByActorID == "") {
		return Invocation{}, invalid("mutations require --idempotency and --actor")
	}
	if kind == QueryInvocation && invocation.Execute {
		return Invocation{}, invalid("queries cannot use --execute")
	}
	sort.Strings(invocation.AcknowledgedWarningCodes)
	invocation.AcknowledgedWarningCodes = compactStrings(invocation.AcknowledgedWarningCodes)
	encoded, err := json.Marshal(arguments)
	if err != nil {
		return Invocation{}, invalid("arguments are not JSON serializable")
	}
	invocation.Arguments = encoded
	return invocation, nil
}

func Run(ctx context.Context, client Client, invocation Invocation, approval *Approval) (Outcome, error) {
	if client == nil {
		return Outcome{}, invalid("HTTP client is required")
	}
	if invocation.Kind == QueryInvocation {
		result, err := client.Query(ctx, QueryRequest{Name: invocation.Name, WorkspaceID: invocation.WorkspaceID, Arguments: invocation.Arguments})
		return Outcome{QueryResult: result}, err
	}
	request := application.CommandRequest{
		Name: invocation.Name, Arguments: invocation.Arguments,
		Envelope: application.CommandEnvelope{
			IdempotencyKey: invocation.IdempotencyKey, ExecutedByActorID: invocation.ExecutedByActorID,
			ExpectedWorkspaceRevision: invocation.ExpectedWorkspaceRevision,
		},
	}
	preview, err := client.Preview(ctx, request)
	if err != nil {
		return Outcome{}, err
	}
	outcome := Outcome{Preview: &preview}
	approvalRequired := hasCode(preview.Errors, domain.CodeHumanApprovalRequired)
	for _, diagnostic := range preview.Errors {
		if diagnostic.Code != domain.CodeHumanApprovalRequired {
			return outcome, &StructuredError{Code: diagnostic.Code, Message: "preview rejected: " + diagnostic.Code}
		}
	}
	missingWarnings := unacknowledged(preview.Warnings, invocation.AcknowledgedWarningCodes)
	if len(missingWarnings) != 0 {
		outcome.WarningAcknowledgementRequired = missingWarnings
		return outcome, nil
	}
	if !invocation.Execute {
		outcome.ApprovalRequired = approvalRequired
		return outcome, nil
	}
	if approvalRequired && (approval == nil || strings.TrimSpace(approval.ApprovedByActorID) == "") {
		outcome.ApprovalRequired = true
		return outcome, nil
	}
	if approvalRequired && approval.ExpectedWorkspaceRevision != preview.ExpectedWorkspaceRevision {
		return outcome, &StructuredError{Code: domain.CodeStaleRevision, Message: "approved preview revision is stale"}
	}
	if approvalRequired && (approval.ApprovedCommandHash != preview.CommandHash || approval.DecisionSnapshotHash != preview.DecisionSnapshotHash) {
		return outcome, &StructuredError{Code: domain.CodeHumanApprovalMismatch, Message: "approved preview does not match current preview"}
	}
	request.Envelope.ExpectedWorkspaceRevision = preview.ExpectedWorkspaceRevision
	request.Envelope.AcknowledgedWarningCodes = append([]string(nil), invocation.AcknowledgedWarningCodes...)
	if approvalRequired {
		request.Envelope.HumanApprovalAttestation = &application.HumanApprovalAttestation{
			ApprovedByActorID: strings.TrimSpace(approval.ApprovedByActorID), ApprovedCommandHash: approval.ApprovedCommandHash,
			DecisionSnapshotHash: approval.DecisionSnapshotHash, StatementHash: strings.TrimSpace(approval.StatementHash), ConversationRef: strings.TrimSpace(approval.ConversationRef),
		}
	}
	execution, err := client.Execute(ctx, request)
	if err != nil {
		return outcome, err
	}
	outcome.Execution = &execution
	return outcome, nil
}

func invalid(message string) error {
	return &StructuredError{Code: "invalid_request", Message: message}
}

func parseValue(value string) any {
	var decoded any
	if json.Unmarshal([]byte(value), &decoded) == nil {
		return decoded
	}
	return value
}

func argumentKey(flag string) string {
	aliases := map[string]string{"lane": "laneId", "task": "taskId", "gate": "gateId", "run": "runId", "record": "recordId"}
	if alias := aliases[flag]; alias != "" {
		return alias
	}
	parts := strings.Split(flag, "-")
	for index := 1; index < len(parts); index++ {
		if parts[index] != "" {
			parts[index] = strings.ToUpper(parts[index][:1]) + parts[index][1:]
		}
	}
	return strings.Join(parts, "")
}

func compactStrings(values []string) []string {
	result := values[:0]
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}

func hasCode(values []domain.Diagnostic, code string) bool {
	for _, value := range values {
		if value.Code == code {
			return true
		}
	}
	return false
}

func unacknowledged(values []domain.Diagnostic, acknowledged []string) []string {
	acknowledgedSet := make(map[string]bool, len(acknowledged))
	for _, code := range acknowledged {
		acknowledgedSet[code] = true
	}
	missing := []string{}
	for _, diagnostic := range values {
		if !acknowledgedSet[diagnostic.Code] {
			missing = append(missing, diagnostic.Code)
		}
	}
	sort.Strings(missing)
	return compactStrings(missing)
}

func IsCode(err error, code string) bool {
	var structured *StructuredError
	return errors.As(err, &structured) && structured.Code == code
}

func (i Invocation) String() string { return fmt.Sprintf("%s %s", i.Kind, i.Name) }
