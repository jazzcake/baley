package application

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jazzcake/baley/server/internal/domain"
)

func TestTaskJournalLifecycleArgumentsDecodeContextNote(t *testing.T) {
	tests := []struct {
		command string
		payload string
	}{
		{"task.create", `{"workspaceId":"w","taskUuid":"u","laneId":"l","phaseId":"p","title":"t","contextNote":{"narrative":"created","context":{"problem":"p"}}}`},
		{"backlog.promote", `{"workspaceId":"w","backlogPublicId":1,"taskUuid":"u","phaseId":"p","contextNote":{"narrative":"promoted"}}`},
		{"run.start", `{"workspaceId":"w","taskId":1,"clientRunId":"r","kind":"implementation","contextNote":{"narrative":"started"}}`},
		{"task.update", `{"workspaceId":"w","taskId":1,"title":"t","contextNote":{"narrative":"updated"}}`},
		{"task.rework", `{"workspaceId":"w","taskId":1,"reason":"r","contextNote":{"narrative":"rework"}}`},
		{"task.block", `{"workspaceId":"w","taskId":1,"reason":"r","contextNote":{"narrative":"blocked"}}`},
		{"task.unblock", `{"workspaceId":"w","taskId":1,"contextNote":{"narrative":"unblocked"}}`},
		{"task.report_implemented", `{"workspaceId":"w","taskId":1,"assessment":"a","contextNote":{"narrative":"implemented"}}`},
		{"task.confirm", `{"workspaceId":"w","taskId":1,"contextNote":{"narrative":"confirmed"}}`},
		{"task.discard", `{"workspaceId":"w","taskId":1,"reason":"r","contextNote":{"narrative":"discarded"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			_, typed, err := decodeArguments(tt.command, json.RawMessage(tt.payload))
			if err != nil {
				t.Fatal(err)
			}
			plan := MutationPlan{TaskID: "task-internal", TaskCreate: &domain.Task{ID: "task-created"}, Run: &domain.Run{TaskID: "task-run"}}
			journal, err := taskJournalFromCommand(tt.command, typed, plan)
			if err != nil || journal == nil || journal.SchemaVersion != 1 {
				t.Fatalf("journal=%+v err=%v", journal, err)
			}
		})
	}
}

func TestTaskJournalMapsLifecycleStageAndSourceEvent(t *testing.T) {
	note := &TaskContextNote{Narrative: " stated ", Context: map[string]any{"goal": "ship"}}
	tests := []struct {
		name, stage, event string
		typed              any
		plan               MutationPlan
	}{
		{"task.create", "created", "task.created", taskCreateArgs{ContextNote: note}, MutationPlan{TaskCreate: &domain.Task{ID: "created"}}},
		{"backlog.promote", "created", "task.created", backlogMutationArgs{ContextNote: note}, MutationPlan{TaskCreate: &domain.Task{ID: "promoted"}}},
		{"run.start", "run_started", "run.started", runStartArgs{ContextNote: note}, MutationPlan{Run: &domain.Run{TaskID: "run-task"}}},
		{"task.update", "updated", "task.updated", taskMutationArgs{ContextNote: note}, MutationPlan{TaskID: "task"}},
		{"task.rework", "rework_started", "task.rework_started", taskMutationArgs{ContextNote: note}, MutationPlan{TaskID: "task"}},
		{"task.block", "blocked", "task.blocked", taskMutationArgs{ContextNote: note}, MutationPlan{TaskID: "task"}},
		{"task.unblock", "unblocked", "task.unblocked", taskMutationArgs{ContextNote: note}, MutationPlan{TaskID: "task"}},
		{"task.report_implemented", "implemented", "task.implemented_reported", taskReportImplementedArgs{ContextNote: note}, MutationPlan{TaskID: "task"}},
		{"task.confirm", "confirmed", "task.confirmed", taskConfirmArgs{ContextNote: note}, MutationPlan{TaskID: "task"}},
		{"task.discard", "discarded", "task.discarded", taskMutationArgs{ContextNote: note}, MutationPlan{TaskID: "task"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			journal, err := taskJournalFromCommand(tt.name, tt.typed, tt.plan)
			if err != nil || journal == nil || journal.LifecycleStage != tt.stage || journal.SourceEventType != tt.event || journal.Narrative != "stated" {
				t.Fatalf("journal=%+v err=%v", journal, err)
			}
		})
	}
}

func TestTaskJournalIsRebuiltFromNormalizedLifecycleEventPayload(t *testing.T) {
	journal, err := taskJournalFromCommand("task.update", taskMutationArgs{ContextNote: &TaskContextNote{
		Narrative: "  Actual operator context.  ",
		Context:   map[string]any{"goal": "ship", "alternatives": []any{"wait", "proceed"}},
	}}, MutationPlan{TaskID: "task-internal"})
	if err != nil || journal == nil || journal.ContextDigest == "" {
		t.Fatalf("journal=%+v err=%v", journal, err)
	}
	events := []EventWrite{{Type: "task.updated", Payload: map[string]any{
		"taskId": "task-internal", "before": map[string]any{"title": "old"}, "after": map[string]any{"title": "new"},
	}}}
	if err = attachTaskJournalToEvent(events, journal); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(events[0].Payload)
	if err != nil {
		t.Fatal(err)
	}
	rebuilt, err := TaskJournalFromEvent(EventProjection{ID: "event", EventType: "task.updated", Payload: payload})
	if err != nil || !TaskJournalsEqual(journal, rebuilt) {
		t.Fatalf("rebuilt=%+v original=%+v err=%v payload=%s", rebuilt, journal, err, payload)
	}
	var persisted map[string]any
	if err = json.Unmarshal(payload, &persisted); err != nil {
		t.Fatal(err)
	}
	seed, ok := persisted["taskJournal"].(map[string]any)
	if !ok || seed["schemaVersion"] != float64(1) || seed["narrative"] != "Actual operator context." || seed["contextDigest"] != journal.ContextDigest {
		t.Fatalf("Event did not persist normalized Task journal seed: %#v", persisted)
	}
	seed["narrative"] = "tampered"
	tampered, _ := json.Marshal(persisted)
	if _, err = TaskJournalFromEvent(EventProjection{ID: "event", EventType: "task.updated", Payload: tampered}); err == nil {
		t.Fatal("Task journal rebuild accepted a payload that no longer matched its digest")
	}
}

func TestTaskJournalSkipsEmptyContextAndEnforcesBounds(t *testing.T) {
	plan := MutationPlan{TaskID: "task"}
	for _, note := range []*TaskContextNote{{}, {Narrative: "  ", Context: map[string]any{"goal": " ", "alternatives": []any{}}}} {
		journal, err := taskJournalFromCommand("task.update", taskMutationArgs{ContextNote: note}, plan)
		if err != nil || journal != nil {
			t.Fatalf("empty journal=%+v err=%v", journal, err)
		}
	}
	for _, note := range []*TaskContextNote{
		{Narrative: strings.Repeat("n", 4001)},
		{Context: map[string]any{"goal": strings.Repeat("x", 16*1024)}},
	} {
		if _, err := taskJournalFromCommand("task.update", taskMutationArgs{ContextNote: note}, plan); err == nil {
			t.Fatalf("oversized note accepted: %#v", note)
		}
	}
}

func TestAbsentContextNotePreservesLegacyCommandHash(t *testing.T) {
	type legacyTaskUpdateArgs struct {
		WorkspaceID    string  `json:"workspaceId"`
		TaskID         int     `json:"taskId"`
		Title          *string `json:"title,omitempty"`
		Description    *string `json:"description,omitempty"`
		CurrentSummary *string `json:"currentSummary,omitempty"`
		Reason         string  `json:"reason,omitempty"`
		TargetPhaseID  string  `json:"targetPhaseId,omitempty"`
	}
	title := "title"
	legacy := legacyTaskUpdateArgs{WorkspaceID: "w", TaskID: 183, Title: &title}
	current := taskMutationArgs{WorkspaceID: "w", TaskID: 183, Title: &title}
	if got, want := hashCommand("task.update", current, 7, "snapshot"), hashCommand("task.update", legacy, 7, "snapshot"); got != want {
		t.Fatalf("absent context hash=%s, legacy=%s", got, want)
	}
	current.ContextNote = &TaskContextNote{Narrative: "actual reason"}
	if got := hashCommand("task.update", current, 7, "snapshot"); got == hashCommand("task.update", legacy, 7, "snapshot") {
		t.Fatal("contextNote did not bind the command hash")
	}
}
