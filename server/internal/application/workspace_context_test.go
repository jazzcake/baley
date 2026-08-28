package application

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"
)

func TestWorkspaceContextOmitsCompletedPhasesAndTaskBodies(t *testing.T) {
	snapshot := Snapshot{
		Workspace: WorkspaceProjection{ID: "w", Revision: 7},
		Lanes:     []LaneProjection{{ID: "server", Name: "Server"}, {ID: "client", Name: "Client"}},
		Phases:    []PhaseProjection{{ID: "done", Name: "Done", State: "completed", Position: 0}, {ID: "active", Name: "Active", State: "active", Position: 1}},
		Tasks:     []TaskProjection{{PublicID: 1, LaneID: "server", PhaseID: "done", Title: "hidden", Description: "must not project", Status: "confirmed"}, {PublicID: 2, LaneID: "server", PhaseID: "active", Title: "also hidden", Description: "must not project", Status: "in_progress"}, {PublicID: 3, LaneID: "client", PhaseID: "active", Status: "pending"}},
	}
	context := WorkspaceContext(snapshot)
	if len(context.Phases) != 1 || context.Phases[0].ID != "active" || !context.FullGraphAvailable || context.Workspace.Revision != 7 {
		t.Fatalf("unexpected context: %#v", context)
	}
	if got := context.Phases[0].LaneCounts[0]; got.LaneID != "client" || got.StatusCounts["pending"] != 1 {
		t.Fatalf("client count=%#v", got)
	}
	if got := context.Phases[0].LaneCounts[1]; got.LaneID != "server" || got.StatusCounts["in_progress"] != 1 {
		t.Fatalf("server count=%#v", got)
	}
	encoded, err := json.Marshal(context)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"hidden", "must not project", "also hidden", `"description"`, `"title"`} {
		if bytes.Contains(encoded, []byte(forbidden)) {
			t.Fatalf("compact context leaked Task detail %q: %s", forbidden, encoded)
		}
	}
}

func TestWorkspaceContextUsesRevisionAsSnapshotMarkerAndStableOrdering(t *testing.T) {
	snapshot := Snapshot{
		Workspace: WorkspaceProjection{ID: "w", Revision: 42},
		Lanes:     []LaneProjection{{ID: "z", Name: "Zulu"}, {ID: "a", Name: "Alpha"}},
		Phases:    []PhaseProjection{{ID: "later", Name: "Later", State: "planned", Position: 2}, {ID: "now", Name: "Now", State: "active", Position: 1}},
		Tasks:     []TaskProjection{{PublicID: 10, LaneID: "z", PhaseID: "now", Status: "pending"}},
	}
	context := WorkspaceContext(snapshot)
	if context.Workspace.Revision != 42 || len(context.Phases) != 2 || context.Phases[0].ID != "now" || context.Phases[1].ID != "later" {
		t.Fatalf("context is not a stable revisioned summary: %#v", context)
	}
	if got := context.Phases[0].LaneCounts; len(got) != 2 || got[0].LaneID != "a" || got[1].LaneID != "z" || got[1].StatusCounts["pending"] != 1 {
		t.Fatalf("lane order or counts=%#v", got)
	}
	snapshot.Workspace.Revision++
	refreshed := WorkspaceContext(snapshot)
	if refreshed.Workspace.Revision != 43 || refreshed.Phases[0].ID != context.Phases[0].ID {
		t.Fatalf("revision change did not produce a refresh marker: %#v", refreshed)
	}
}

func TestPhaseTasksPageIsBoundedAndCursorOrdered(t *testing.T) {
	snapshot := Snapshot{Tasks: []TaskProjection{{PublicID: 30, PhaseID: "p"}, {PublicID: 10, PhaseID: "p"}, {PublicID: 20, PhaseID: "p"}, {PublicID: 40, PhaseID: "other"}}}
	page, cursor, more := PhaseTasksPage(snapshot, "p", 10, 1)
	if len(page) != 1 || page[0].PublicID != 20 || cursor != 20 || !more {
		t.Fatalf("page=%#v cursor=%d more=%v", page, cursor, more)
	}
}

func TestPhaseTasksPageKeepsCursorsScopedToTheSelectedPhase(t *testing.T) {
	snapshot := Snapshot{Tasks: []TaskProjection{
		{PublicID: 1, PhaseID: "a", Title: "a-one"},
		{PublicID: 3, PhaseID: "a", Title: "a-three"},
		{PublicID: 2, PhaseID: "b", Title: "b-two"},
		{PublicID: 4, PhaseID: "b", Title: "b-four"},
	}}
	page, cursor, more := PhaseTasksPage(snapshot, "b", 1, 1)
	if len(page) != 1 || page[0].PublicID != 2 || page[0].Title != "b-two" || cursor != 2 || !more {
		t.Fatalf("phase B page=%#v cursor=%d more=%v", page, cursor, more)
	}
	page, cursor, more = PhaseTasksPage(snapshot, "a", 2, 100)
	if len(page) != 1 || page[0].PublicID != 3 || cursor != 0 || more {
		t.Fatalf("phase A page=%#v cursor=%d more=%v", page, cursor, more)
	}
}

func BenchmarkWorkspaceContextScale(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("tasks=%d", size), func(b *testing.B) {
			snapshot := benchmarkWorkspaceSnapshot(size)
			var encoded []byte
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				value := WorkspaceContext(snapshot)
				var err error
				encoded, err = json.Marshal(value)
				if err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(len(encoded)), "response-bytes")
		})
	}
}

func BenchmarkWorkspaceGraphScale(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("tasks=%d", size), func(b *testing.B) {
			snapshot := benchmarkWorkspaceSnapshot(size)
			fullGraph := benchmarkFullGraph(snapshot)
			var encoded []byte
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var err error
				encoded, err = json.Marshal(fullGraph)
				if err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(len(encoded)), "response-bytes")
		})
	}
}

func TestWorkspaceContextScaleMeasurements(t *testing.T) {
	for _, size := range []int{100, 1000, 10000} {
		snapshot := benchmarkWorkspaceSnapshot(size)
		samples := make([]int64, 0, 101)
		responseBytes := 0
		fullGraphBytes := 0
		const operationsPerSample = 100
		for sample := 0; sample < cap(samples); sample++ {
			started := time.Now()
			for operation := 0; operation < operationsPerSample; operation++ {
				value := WorkspaceContext(snapshot)
				encoded, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				responseBytes = len(encoded)
			}
			samples = append(samples, time.Since(started).Nanoseconds()/operationsPerSample)
		}
		fullGraph, err := json.Marshal(benchmarkFullGraph(snapshot))
		if err != nil {
			t.Fatal(err)
		}
		fullGraphBytes = len(fullGraph)
		sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
		p95 := time.Duration(samples[(len(samples)*95+99)/100-1])
		t.Logf("tasks=%d compact-response-bytes=%d full-graph-bytes=%d p95=%s", size, responseBytes, fullGraphBytes, p95)
		if responseBytes > 1024 {
			t.Fatalf("tasks=%d compact response grew beyond 1 KiB: %d", size, responseBytes)
		}
		if responseBytes >= fullGraphBytes {
			t.Fatalf("tasks=%d compact response is not smaller than full graph: %d >= %d", size, responseBytes, fullGraphBytes)
		}
	}
}

func benchmarkFullGraph(snapshot Snapshot) map[string]any {
	return map[string]any{"workspace": snapshot.Workspace, "phases": snapshot.Phases, "lanes": snapshot.Lanes, "tasks": snapshot.Tasks}
}

func benchmarkWorkspaceSnapshot(size int) Snapshot {
	snapshot := Snapshot{
		Workspace: WorkspaceProjection{ID: "benchmark", Revision: 1},
		Lanes:     []LaneProjection{{ID: "server", Name: "Server"}, {ID: "client", Name: "Client"}},
		Phases:    []PhaseProjection{{ID: "active", Name: "Active", State: "active", Position: 1}, {ID: "done", Name: "Done", State: "completed", Position: 0}},
		Tasks:     make([]TaskProjection, 0, size),
	}
	for index := 0; index < size; index++ {
		phaseID := "active"
		if index%4 == 0 {
			phaseID = "done"
		}
		snapshot.Tasks = append(snapshot.Tasks, TaskProjection{PublicID: index + 1, PhaseID: phaseID, LaneID: []string{"server", "client"}[index%2], Title: fmt.Sprintf("Task %d", index), Description: "This must not be encoded by the compact context path.", Status: []string{"pending", "in_progress", "implemented", "confirmed"}[index%4]})
	}
	return snapshot
}
