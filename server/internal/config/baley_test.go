package config

import (
	"errors"
	"strings"
	"testing"
)

const (
	workspaceID  = "018f4c18-7a3b-7cc1-8b1a-b9f8f03f4511"
	repositoryID = "018f4c18-7a3b-7cc1-8b1a-b9f8f03f4512"
	recordRepoID = "018f4c18-7a3b-7cc1-8b1a-b9f8f03f4513"
)

func TestParseMinimalAndRoundTrip(t *testing.T) {
	data := []byte("version: 1\nserver: https://baley.example.com/\nworkspace_id: " + workspaceID + "\nrepository_id: " + repositoryID + "\ntask_records:\n  root: task-records/./\n")
	config, err := Parse(data)
	if err != nil || config.Server != "https://baley.example.com" || config.TaskRecords.Root != "task-records" || config.EffectiveRecordRepositoryID() != repositoryID {
		t.Fatalf("minimal config failed: %+v %v", config, err)
	}
	encoded, err := Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	roundTrip, err := Parse(encoded)
	if err != nil || roundTrip != config {
		t.Fatalf("round trip drift: %+v %+v %v", config, roundTrip, err)
	}
}

func TestMultiRepositoryConfigSelectsRecordRepository(t *testing.T) {
	config, err := Parse([]byte("version: 1\nserver: http://127.0.0.1:8080\nworkspace_id: " + workspaceID + "\nrepository_id: " + repositoryID + "\nrecord_repository_id: " + recordRepoID + "\ntask_records:\n  root: project-records\n"))
	if err != nil || config.EffectiveRecordRepositoryID() != recordRepoID {
		t.Fatalf("multi-repository config failed: %+v %v", config, err)
	}
}

func TestRejectsUnknownVersionMissingAndUnknownFields(t *testing.T) {
	base := "server: https://baley.example.com\nworkspace_id: " + workspaceID + "\nrepository_id: " + repositoryID + "\ntask_records:\n  root: task-records\n"
	assertConfigCode(t, []byte("version: 2\n"+base), CodeUnsupportedVersion)
	assertConfigCode(t, []byte("version: 1\nserver: https://baley.example.com\nrepository_id: "+repositoryID+"\ntask_records:\n  root: task-records\n"), CodeInvalidConfig)
	assertConfigCode(t, []byte("version: 1\n"+base+"unknown: value\n"), CodeInvalidConfig)
	assertConfigCode(t, []byte("version: 1\n"+base+"---\nversion: 1\n"), CodeInvalidConfig)
}

func TestRejectsSecretLikeFieldsAtAnyDepth(t *testing.T) {
	base := "version: 1\nserver: https://baley.example.com\nworkspace_id: " + workspaceID + "\nrepository_id: " + repositoryID + "\ntask_records:\n  root: task-records\n"
	for _, extra := range []string{"api_token: value\n", "auth:\n  client_secret: value\n", "metadata:\n  password_hint: value\n", "authorization: Bearer value\n"} {
		t.Run(strings.Fields(extra)[0], func(t *testing.T) {
			assertConfigCode(t, []byte(base+extra), CodeSensitiveField)
		})
	}
	assertConfigCode(t, []byte("version: 1\nserver: https://user:pass@baley.example.com\nworkspace_id: "+workspaceID+"\nrepository_id: "+repositoryID+"\ntask_records:\n  root: task-records\n"), CodeInvalidConfig)
}

func TestRejectsYAMLAliasesAnchorsAndMergeKeys(t *testing.T) {
	base := "version: 1\nserver: https://baley.example.com\nworkspace_id: " + workspaceID + "\nrepository_id: " + repositoryID + "\n"
	for _, taskRecords := range []string{
		"defaults: &records\n  root: task-records\ntask_records: *records\n",
		"defaults: &records\n  root: task-records\ntask_records:\n  <<: *records\n",
	} {
		assertConfigCode(t, []byte(base+taskRecords), CodeInvalidConfig)
	}
}

func TestAllowsMergeTokenAsOrdinaryScalarValue(t *testing.T) {
	config, err := Parse([]byte("version: 1\nserver: https://baley.example.com\nworkspace_id: " + workspaceID + "\nrepository_id: " + repositoryID + "\ntask_records:\n  root: \"<<\"\n"))
	if err != nil || config.TaskRecords.Root != "<<" {
		t.Fatalf("ordinary merge-token scalar rejected: %+v %v", config, err)
	}
}

func TestRejectsInvalidTaskRecordRootAndServer(t *testing.T) {
	valid := BaleyConfig{Version: 1, Server: "https://baley.example.com", WorkspaceID: workspaceID, RepositoryID: repositoryID, TaskRecords: TaskRecordsConfig{Root: "task-records"}}
	for _, root := range []string{"", ".", "../records", "records/../other", "/tmp/records", `C:\records`, "file:records", "records\x00hidden"} {
		t.Run(root, func(t *testing.T) {
			candidate := valid
			candidate.TaskRecords.Root = strings.ReplaceAll(root, `\x00`, "\x00")
			_, err := Normalize(candidate)
			assertErrorCode(t, err, CodeInvalidPath)
		})
	}
	for _, server := range []string{"", "baley.example.com", "file:///tmp/baley", "https://user@baley.example.com", "https://baley.example.com/path", "https://baley.example.com?token=x", "https://:443", "https://baley.example.com:0", "https://baley.example.com:65536"} {
		candidate := valid
		candidate.Server = server
		_, err := Normalize(candidate)
		assertErrorCode(t, err, CodeInvalidConfig)
	}
}

func assertConfigCode(t *testing.T, data []byte, code ErrorCode) {
	t.Helper()
	_, err := Parse(data)
	assertErrorCode(t, err, code)
}

func assertErrorCode(t *testing.T, err error, code ErrorCode) {
	t.Helper()
	var configErr *ConfigError
	if !errors.As(err, &configErr) || configErr.Code != code {
		t.Fatalf("want %s, got %v", code, err)
	}
}
