package application

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBacklogCommandArgumentsRejectFieldsOwnedByOtherCommands(t *testing.T) {
	tests := []struct {
		name    string
		command string
		payload string
		field   string
	}{
		{
			name:    "create rejects phase targeting",
			command: "backlog.create",
			payload: `{"workspaceId":"w","backlogUuid":"b","laneId":"l","title":"idea","phaseId":"p"}`,
			field:   "phaseId",
		},
		{
			name:    "promote rejects lane override",
			command: "backlog.promote",
			payload: `{"workspaceId":"w","backlogPublicId":1,"taskUuid":"t","phaseId":"p","laneId":"other"}`,
			field:   "laneId",
		},
		{
			name:    "promote rejects title override",
			command: "backlog.promote",
			payload: `{"workspaceId":"w","backlogPublicId":1,"taskUuid":"t","phaseId":"p","title":"override"}`,
			field:   "title",
		},
		{
			name:    "promote rejects description override",
			command: "backlog.promote",
			payload: `{"workspaceId":"w","backlogPublicId":1,"taskUuid":"t","phaseId":"p","description":"override"}`,
			field:   "description",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := decodeArguments(tt.command, json.RawMessage(tt.payload))
			if err == nil || !strings.Contains(err.Error(), "unknown field") || !strings.Contains(err.Error(), tt.field) {
				t.Fatalf("decode error=%v, want unknown field %q", err, tt.field)
			}
		})
	}
}

func TestBacklogCreateAndPromoteDecodeToInternalArguments(t *testing.T) {
	_, createRaw, err := decodeArguments("backlog.create", json.RawMessage(
		`{"workspaceId":"w","backlogUuid":"b","laneId":"l","title":"idea","description":"detail"}`,
	))
	if err != nil {
		t.Fatal(err)
	}
	create := createRaw.(backlogMutationArgs)
	if create.PhaseID != "" || create.Title == nil || *create.Title != "idea" {
		t.Fatalf("create args=%+v", create)
	}

	_, promoteRaw, err := decodeArguments("backlog.promote", json.RawMessage(
		`{"workspaceId":"w","backlogPublicId":1,"taskUuid":"t","phaseId":"p","predecessorTaskIds":[2]}`,
	))
	if err != nil {
		t.Fatal(err)
	}
	promote := promoteRaw.(backlogMutationArgs)
	if promote.LaneID != "" || promote.Title != nil || promote.Description != nil || promote.PhaseID != "p" {
		t.Fatalf("promote args=%+v", promote)
	}
}
