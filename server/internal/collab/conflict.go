package collab

import (
	"sort"
	"strings"

	"github.com/jazzcake/baley/server/internal/domain"
)

type ConflictKind string

const (
	WorkspaceRevisionConflict ConflictKind = "workspace_revision_conflict"
	RunVersionConflict        ConflictKind = "run_version_conflict"
	GraphConflict             ConflictKind = "graph_conflict"
	IdempotencyConflict       ConflictKind = "idempotency_conflict"
	RecordIdentityConflict    ConflictKind = "record_identity_conflict"
)

type RetryDisposition string

const (
	ReloadAndReplan     RetryDisposition = "reload_and_replan"
	ReloadRun           RetryDisposition = "reload_run_and_replan"
	ReplanGraph         RetryDisposition = "reload_graph_and_replan"
	ReviewNewKey        RetryDisposition = "review_intent_before_new_idempotency_key"
	CreateRecordVersion RetryDisposition = "review_and_create_new_record_version"
)

type ConflictInput struct {
	Code                      string
	WorkspaceID               string
	EntityID                  string
	ExpectedWorkspaceRevision int64
	CurrentWorkspaceRevision  int64
	ExpectedRunVersion        int64
	CurrentRunVersion         int64
	IdempotencyKey            string
	ChangedEntityIDs          []string
}

type ConflictResult struct {
	Code                      string           `json:"code"`
	Kind                      ConflictKind     `json:"kind"`
	EntityID                  string           `json:"entityId,omitempty"`
	CurrentWorkspaceRevision  int64            `json:"currentWorkspaceRevision,omitempty"`
	CurrentRunVersion         int64            `json:"currentRunVersion,omitempty"`
	ReloadQuery               string           `json:"reloadQuery,omitempty"`
	ChangedEntityIDs          []string         `json:"changedEntityIds,omitempty"`
	Disposition               RetryDisposition `json:"disposition"`
	RetryableAfterReload      bool             `json:"retryableAfterReload"`
	SafeToRetrySameRequest    bool             `json:"safeToRetrySameRequest"`
	RequiresReplan            bool             `json:"requiresReplan"`
	RequiresNewIdempotencyKey bool             `json:"requiresNewIdempotencyKey"`
	RequiresNewRecordVersion  bool             `json:"requiresNewRecordVersion"`
}

func BuildConflict(input ConflictInput) (ConflictResult, error) {
	result := ConflictResult{Code: input.Code, EntityID: input.EntityID}
	switch input.Code {
	case domain.CodeStaleRevision:
		if strings.TrimSpace(input.WorkspaceID) == "" || input.ExpectedWorkspaceRevision <= 0 || input.CurrentWorkspaceRevision <= input.ExpectedWorkspaceRevision {
			return ConflictResult{}, ErrInvalidConflict
		}
		result.Kind = WorkspaceRevisionConflict
		result.CurrentWorkspaceRevision = input.CurrentWorkspaceRevision
		result.ReloadQuery = "workspace.get"
		result.Disposition = ReloadAndReplan
		result.RetryableAfterReload = true
		result.RequiresReplan = true
		result.ChangedEntityIDs = normalizedIDs(input.ChangedEntityIDs)
	case domain.CodeStaleRunVersion:
		if strings.TrimSpace(input.EntityID) == "" || input.ExpectedRunVersion <= 0 || input.CurrentRunVersion <= input.ExpectedRunVersion {
			return ConflictResult{}, ErrInvalidConflict
		}
		result.Kind = RunVersionConflict
		result.CurrentRunVersion = input.CurrentRunVersion
		result.ReloadQuery = "run.list"
		result.Disposition = ReloadRun
		result.RetryableAfterReload = true
		result.RequiresReplan = true
	case domain.CodeDependencyCycle, domain.CodeDuplicateDependency, domain.CodeInvalidDependencyPatch, domain.CodeTerminalPathConflict, domain.CodeCrossWorkspaceDependency:
		if strings.TrimSpace(input.WorkspaceID) == "" || input.CurrentWorkspaceRevision <= 0 {
			return ConflictResult{}, ErrInvalidConflict
		}
		result.Kind = GraphConflict
		result.CurrentWorkspaceRevision = input.CurrentWorkspaceRevision
		result.ReloadQuery = "workspace.graph"
		result.Disposition = ReplanGraph
		result.RetryableAfterReload = true
		result.RequiresReplan = true
		result.ChangedEntityIDs = normalizedIDs(input.ChangedEntityIDs)
	case domain.CodeIdempotencyConflict:
		if strings.TrimSpace(input.IdempotencyKey) == "" {
			return ConflictResult{}, ErrInvalidConflict
		}
		result.Kind = IdempotencyConflict
		result.Disposition = ReviewNewKey
		result.RequiresNewIdempotencyKey = true
	case domain.CodeRecordHashConflict:
		if strings.TrimSpace(input.EntityID) == "" {
			return ConflictResult{}, ErrInvalidConflict
		}
		result.Kind = RecordIdentityConflict
		result.ReloadQuery = "record.list"
		result.Disposition = CreateRecordVersion
		result.RequiresNewRecordVersion = true
	default:
		return ConflictResult{}, ErrUnsupportedConflict
	}
	return result, nil
}

type conflictError string

func (e conflictError) Error() string { return string(e) }

const (
	ErrInvalidConflict     conflictError = "invalid conflict input"
	ErrUnsupportedConflict conflictError = "unsupported conflict code"
)

func normalizedIDs(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}
