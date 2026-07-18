package collab

import (
	"reflect"
	"testing"
	"time"

	"github.com/jazzcake/baley/server/internal/domain"
)

func TestDetectNotificationsThresholdsStableFingerprintsAndResolution(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	old := now.Add(-24 * time.Hour)
	heartbeat := now.Add(-25 * time.Hour)
	blocked := old
	input := NotificationInput{
		WorkspaceID:    "workspace",
		Now:            now,
		LaneStaleAfter: 24 * time.Hour,
		BlockerAfter:   24 * time.Hour,
		Lanes: []ObservedLane{{
			Lane:      domain.Lane{ID: "lane", WorkspaceID: "workspace", State: domain.LaneActive},
			UpdatedAt: old,
		}},
		Tasks: []ObservedTask{
			{Task: domain.Task{ID: "blocked", WorkspaceID: "workspace", Status: domain.TaskInProgress, BlockedAt: &blocked, BlockerReason: "waiting"}, StateSince: old},
			{Task: domain.Task{ID: "implemented", WorkspaceID: "workspace", Status: domain.TaskImplemented}, StateSince: old},
		},
		Gates: []ObservedGate{{
			Gate:                 domain.Gate{ID: "gate", WorkspaceID: "workspace", CriteriaRevision: 2},
			ReadySince:           old,
			ConditionSatisfiedAt: map[string]time.Time{"link": old},
			Conditions:           []domain.GateTaskCondition{{WorkspaceID: "workspace", GateID: "gate", LinkID: "link", TaskID: "implemented", TaskStatus: domain.TaskImplemented, Passed: true, PassReason: "waived"}},
		}},
		Runs: []domain.Run{{
			ID: "run", WorkspaceID: "workspace", TaskID: "implemented", Kind: domain.RunImplementation, Status: domain.RunRunning,
			StartedAt: heartbeat.Add(-time.Hour), HeartbeatAt: heartbeat, LeaseExpiresAt: old, Version: 3,
		}},
	}

	first, err := DetectNotifications(input)
	if err != nil || len(first) != 5 {
		t.Fatalf("notification projection wrong: %+v %v", first, err)
	}
	input.Now = input.Now.Add(2 * time.Hour)
	second, err := DetectNotifications(input)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatalf("fingerprints changed across scans: first=%+v second=%+v err=%v", first, second, err)
	}

	input.Lanes[0].Lane.State = domain.LaneClosedOut
	input.Tasks[0].Task.BlockedAt = nil
	input.Tasks[0].Task.BlockerReason = ""
	input.Tasks[1].Task.Status = domain.TaskConfirmed
	input.Gates[0].Conditions[0].TaskStatus = domain.TaskConfirmed
	passed := now
	input.Gates[0].Gate.PassedAt = &passed
	input.Runs[0].Status = domain.RunSucceeded
	input.Runs[0].EndedAt = &passed
	input.Runs[0].ResultSummary = "completed"
	resolved, err := DetectNotifications(input)
	if err != nil || len(resolved) != 0 {
		t.Fatalf("resolved candidates remained: %+v %v", resolved, err)
	}
}

func TestDetectNotificationsAllowsOpenGateWithoutReadySince(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	input := NotificationInput{
		WorkspaceID: "w", Now: now, LaneStaleAfter: time.Hour, BlockerAfter: time.Hour,
		Tasks: []ObservedTask{{Task: domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskPending}, StateSince: now}},
		Gates: []ObservedGate{{
			Gate:       domain.Gate{ID: "g", WorkspaceID: "w", CriteriaRevision: 1},
			Conditions: []domain.GateTaskCondition{{WorkspaceID: "w", GateID: "g", LinkID: "l", TaskID: "t", TaskStatus: domain.TaskPending}},
		}},
	}
	values, err := DetectNotifications(input)
	if err != nil || len(values) != 0 {
		t.Fatalf("open gate rejected or notified: %+v %v", values, err)
	}
}

func TestDetectNotificationsRejectsStaleGateAndRunTaskSnapshots(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	input := NotificationInput{
		WorkspaceID: "w", Now: now, LaneStaleAfter: time.Hour, BlockerAfter: time.Hour,
		Tasks: []ObservedTask{{Task: domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskPending}, StateSince: now}},
		Gates: []ObservedGate{{
			Gate: domain.Gate{ID: "g", WorkspaceID: "w", CriteriaRevision: 1}, ReadySince: now,
			ConditionSatisfiedAt: map[string]time.Time{"l": now}, Conditions: []domain.GateTaskCondition{{WorkspaceID: "w", GateID: "g", LinkID: "l", TaskID: "t", TaskStatus: domain.TaskConfirmed}},
		}},
	}
	if _, err := DetectNotifications(input); err == nil {
		t.Fatal("gate condition drift from canonical task accepted")
	}
	input.Gates = nil
	input.Runs = []domain.Run{{
		ID: "r", WorkspaceID: "w", TaskID: "missing", Kind: domain.RunImplementation, Status: domain.RunRunning, Version: 1,
		StartedAt: now.Add(-time.Hour), HeartbeatAt: now.Add(-time.Minute), LeaseExpiresAt: now.Add(time.Minute),
	}}
	if _, err := DetectNotifications(input); err == nil {
		t.Fatal("run referencing absent canonical task accepted")
	}
}

func TestDetectNotificationsBindsReadySinceToLastSatisfiedCondition(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	confirmedAt := now.Add(-time.Minute)
	input := NotificationInput{
		WorkspaceID: "w", Now: now, LaneStaleAfter: time.Hour, BlockerAfter: time.Hour,
		Tasks: []ObservedTask{{Task: domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskConfirmed}, StateSince: confirmedAt}},
		Gates: []ObservedGate{{
			Gate: domain.Gate{ID: "g", WorkspaceID: "w", CriteriaRevision: 1}, ReadySince: confirmedAt.Add(-time.Minute),
			ConditionSatisfiedAt: map[string]time.Time{"l": confirmedAt}, Conditions: []domain.GateTaskCondition{{WorkspaceID: "w", GateID: "g", LinkID: "l", TaskID: "t", TaskStatus: domain.TaskConfirmed}},
		}},
	}
	if _, err := DetectNotifications(input); err == nil {
		t.Fatal("ready timestamp before final condition accepted")
	}
	input.Gates[0].ReadySince = confirmedAt
	values, err := DetectNotifications(input)
	if err != nil || len(values) != 1 || values[0].Since != confirmedAt {
		t.Fatalf("canonical ready timestamp rejected: %+v %v", values, err)
	}
}

func TestDetectNotificationsRejectsBeforeThresholdFutureClockAndDuplicates(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	input := NotificationInput{
		WorkspaceID:    "w",
		Now:            now,
		LaneStaleAfter: time.Hour,
		BlockerAfter:   time.Hour,
		Lanes: []ObservedLane{{
			Lane:      domain.Lane{ID: "l", WorkspaceID: "w", State: domain.LaneActive},
			UpdatedAt: now.Add(-time.Hour + time.Nanosecond),
		}},
	}
	values, err := DetectNotifications(input)
	if err != nil || len(values) != 0 {
		t.Fatalf("before threshold fired: %+v %v", values, err)
	}

	input.Lanes[0].UpdatedAt = now.Add(time.Minute)
	if _, err := DetectNotifications(input); err == nil {
		t.Fatal("future observation accepted")
	}
	input.Lanes[0].UpdatedAt = now
	input.Lanes = append(input.Lanes, input.Lanes[0])
	if _, err := DetectNotifications(input); err == nil {
		t.Fatal("duplicate lane accepted")
	}
}

func TestDetectNotificationsRejectsMalformedOwnedEntities(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	base := NotificationInput{WorkspaceID: "w", Now: now, LaneStaleAfter: time.Hour, BlockerAfter: time.Hour}
	tests := []struct {
		name  string
		input NotificationInput
	}{
		{name: "blocked task without reason", input: func() NotificationInput {
			value := base
			blocked := now.Add(-time.Hour)
			value.Tasks = []ObservedTask{{Task: domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskInProgress, BlockedAt: &blocked}, StateSince: blocked}}
			return value
		}()},
		{name: "foreign gate condition", input: func() NotificationInput {
			value := base
			value.Gates = []ObservedGate{{Gate: domain.Gate{ID: "g", WorkspaceID: "w", CriteriaRevision: 1}, ReadySince: now, Conditions: []domain.GateTaskCondition{{WorkspaceID: "other", GateID: "g", LinkID: "l", TaskID: "t", TaskStatus: domain.TaskConfirmed}}}}
			return value
		}()},
		{name: "malformed running run", input: func() NotificationInput {
			value := base
			value.Runs = []domain.Run{{ID: "r", WorkspaceID: "w", Status: domain.RunRunning, LeaseExpiresAt: now.Add(-time.Hour)}}
			return value
		}()},
		{name: "future running run clock", input: func() NotificationInput {
			value := base
			value.Tasks = []ObservedTask{{Task: domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskInProgress}, StateSince: now}}
			value.Runs = []domain.Run{{ID: "r", WorkspaceID: "w", TaskID: "t", Kind: domain.RunImplementation, Status: domain.RunRunning, Version: 1, StartedAt: now.Add(time.Minute), HeartbeatAt: now.Add(time.Minute), LeaseExpiresAt: now.Add(2 * time.Minute)}}
			return value
		}()},
		{name: "terminal run without end", input: func() NotificationInput {
			value := base
			value.Tasks = []ObservedTask{{Task: domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskImplemented}, StateSince: now}}
			value.Runs = []domain.Run{{ID: "r", WorkspaceID: "w", TaskID: "t", Kind: domain.RunImplementation, Status: domain.RunSucceeded, Version: 2, StartedAt: now.Add(-2 * time.Hour), HeartbeatAt: now.Add(-time.Hour), LeaseExpiresAt: now}}
			return value
		}()},
		{name: "whitespace workspace", input: func() NotificationInput {
			value := base
			value.WorkspaceID = " w "
			return value
		}()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := DetectNotifications(test.input); err == nil {
				t.Fatal("malformed entity accepted")
			}
		})
	}
}

func TestDetectNotificationsEnforcesRunSummaryInvariants(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	ended := now.Add(-time.Minute)
	base := NotificationInput{
		WorkspaceID: "w", Now: now, LaneStaleAfter: time.Hour, BlockerAfter: time.Hour,
		Tasks: []ObservedTask{{Task: domain.Task{ID: "t", WorkspaceID: "w", Status: domain.TaskImplemented}, StateSince: now.Add(-time.Hour)}},
	}
	run := domain.Run{ID: "r", WorkspaceID: "w", TaskID: "t", Kind: domain.RunImplementation, Status: domain.RunSucceeded, Version: 2, StartedAt: now.Add(-2 * time.Hour), HeartbeatAt: now.Add(-time.Hour), LeaseExpiresAt: now.Add(-30 * time.Minute), EndedAt: &ended}
	base.Runs = []domain.Run{run}
	if _, err := DetectNotifications(base); err == nil {
		t.Fatal("successful terminal run without result summary accepted")
	}
	base.Runs[0].ResultSummary = "done"
	if _, err := DetectNotifications(base); err != nil {
		t.Fatalf("valid successful run rejected: %v", err)
	}
	base.Runs[0].Status, base.Runs[0].ResultSummary = domain.RunFailed, ""
	if _, err := DetectNotifications(base); err == nil {
		t.Fatal("failed terminal run without error summary accepted")
	}
	base.Runs[0].ErrorSummary = "failed"
	if _, err := DetectNotifications(base); err != nil {
		t.Fatalf("valid failed run rejected: %v", err)
	}
}
