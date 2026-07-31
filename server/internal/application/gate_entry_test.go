package application

import "testing"

func TestGateDecisionSnapshotHashBindsResolvedEntryTasks(t *testing.T) {
	snapshot := Snapshot{Workspace: WorkspaceProjection{ID: "workspace", Revision: 7}}
	gate := GateProjection{
		ID: "ready", FromPhaseID: "from", ToPhaseID: "to", CriteriaRevision: 2,
		EntryTasks: []GateEntryTaskProjection{{GateID: "ready", TaskID: "task-a", SelectionSource: "automatic"}},
	}
	automatic := DecisionSnapshotHash(snapshot, gate)
	gate.EntryTasks = []GateEntryTaskProjection{{GateID: "ready", TaskID: "task-a", SelectionSource: "explicit"}}
	explicit := DecisionSnapshotHash(snapshot, gate)
	if automatic == explicit {
		t.Fatal("decision snapshot hash did not bind entry selection source")
	}
	gate.EntryTasks = []GateEntryTaskProjection{{GateID: "ready", TaskID: "task-b", SelectionSource: "explicit"}}
	if explicit == DecisionSnapshotHash(snapshot, gate) {
		t.Fatal("decision snapshot hash did not bind resolved entry task")
	}
}
