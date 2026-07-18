package collab

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jazzcake/baley/server/internal/domain"
)

func TestWorkspaceRevisionConflictReturnsCurrentRevisionAndReloadHint(t *testing.T) {
	result, err := BuildConflict(ConflictInput{
		Code: domain.CodeStaleRevision, WorkspaceID: "workspace", EntityID: "lane-a",
		ExpectedWorkspaceRevision: 10, CurrentWorkspaceRevision: 11, ChangedEntityIDs: []string{"lane-b", "lane-b"},
	})
	if err != nil || result.Kind != WorkspaceRevisionConflict || result.CurrentWorkspaceRevision != 11 || result.ReloadQuery != "workspace.get" || !result.RetryableAfterReload || !result.RequiresReplan || result.SafeToRetrySameRequest || len(result.ChangedEntityIDs) != 1 {
		t.Fatalf("stale Workspace result wrong: %+v %v", result, err)
	}
}

func TestDifferentLaneWritesStillConflictOnSameWorkspaceRevision(t *testing.T) {
	firstRevision := int64(20)
	result, err := BuildConflict(ConflictInput{
		Code: domain.CodeStaleRevision, WorkspaceID: "workspace", EntityID: "lane-two",
		ExpectedWorkspaceRevision: firstRevision, CurrentWorkspaceRevision: firstRevision + 1,
		ChangedEntityIDs: []string{"lane-one"},
	})
	if err != nil || result.Kind != WorkspaceRevisionConflict || result.EntityID != "lane-two" || result.ChangedEntityIDs[0] != "lane-one" {
		t.Fatalf("cross-Lane revision race hidden: %+v %v", result, err)
	}
}

func TestRunAndGraphConflictsHaveDifferentReloadAndRetryModels(t *testing.T) {
	run, err := BuildConflict(ConflictInput{Code: domain.CodeStaleRunVersion, EntityID: "run", ExpectedRunVersion: 3, CurrentRunVersion: 4})
	if err != nil || run.Kind != RunVersionConflict || run.ReloadQuery != "run.list" || run.CurrentRunVersion != 4 || run.Disposition != ReloadRun {
		t.Fatalf("Run conflict wrong: %+v %v", run, err)
	}
	graph, err := BuildConflict(ConflictInput{Code: domain.CodeInvalidDependencyPatch, WorkspaceID: "workspace", CurrentWorkspaceRevision: 8, ChangedEntityIDs: []string{"task-b", "task-a"}})
	if err != nil || graph.Kind != GraphConflict || graph.ReloadQuery != "workspace.graph" || graph.Disposition != ReplanGraph || strings.Join(graph.ChangedEntityIDs, ",") != "task-a,task-b" {
		t.Fatalf("graph conflict wrong: %+v %v", graph, err)
	}
}

func TestIdempotencyConflictDoesNotLeakKeyOrClaimSafeRetry(t *testing.T) {
	secretKey := "client-private-retry-key"
	result, err := BuildConflict(ConflictInput{Code: domain.CodeIdempotencyConflict, EntityID: "command", IdempotencyKey: secretKey})
	encoded, marshalErr := json.Marshal(result)
	if err != nil || marshalErr != nil || result.Kind != IdempotencyConflict || result.RetryableAfterReload || result.SafeToRetrySameRequest || !result.RequiresNewIdempotencyKey || strings.Contains(string(encoded), secretKey) {
		t.Fatalf("unsafe idempotency result: %+v %s %v %v", result, encoded, err, marshalErr)
	}
}

func TestRecordHashConflictRequiresReviewedNewVersion(t *testing.T) {
	result, err := BuildConflict(ConflictInput{Code: domain.CodeRecordHashConflict, EntityID: "record"})
	if err != nil || result.Kind != RecordIdentityConflict || result.ReloadQuery != "record.list" || result.Disposition != CreateRecordVersion || !result.RequiresNewRecordVersion || result.SafeToRetrySameRequest {
		t.Fatalf("Record hash conflict retry policy wrong: %+v %v", result, err)
	}
}

func TestBuildConflictRejectsUnknownOrIncoherentInputs(t *testing.T) {
	tests := []ConflictInput{
		{Code: "unknown"},
		{Code: domain.CodeStaleRevision, WorkspaceID: "workspace", ExpectedWorkspaceRevision: 1, CurrentWorkspaceRevision: 1},
		{Code: domain.CodeStaleRunVersion, EntityID: "run", ExpectedRunVersion: 1, CurrentRunVersion: 1},
		{Code: domain.CodeStaleRevision, WorkspaceID: "workspace", ExpectedWorkspaceRevision: 2, CurrentWorkspaceRevision: 1},
		{Code: domain.CodeInvalidDependencyPatch, WorkspaceID: "workspace"},
		{Code: domain.CodeIdempotencyConflict},
		{Code: domain.CodeRecordHashConflict},
	}
	for _, input := range tests {
		if _, err := BuildConflict(input); err == nil {
			t.Errorf("invalid conflict accepted: %+v", input)
		}
	}
}
