package gitmeta

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeResponse struct {
	result CommandResult
	err    error
}

type fakeRunner struct {
	responses []fakeResponse
	calls     [][]string
}

func (f *fakeRunner) Run(_ context.Context, directory string, args ...string) (CommandResult, error) {
	f.calls = append(f.calls, append([]string{directory}, args...))
	if len(f.responses) == 0 {
		return CommandResult{}, errors.New("unexpected command")
	}
	response := f.responses[0]
	f.responses = f.responses[1:]
	return response.result, response.err
}

func TestCollectParsesHeadBranchDirtyAndFileMetadata(t *testing.T) {
	runner := &fakeRunner{responses: []fakeResponse{
		{result: commandOutput(repeat("A", 40))},
		{result: commandOutput("feature/wave-5")},
		{result: commandOutput(" M task-records/report.md\x00")},
		{result: commandOutput(repeat("B", 40))},
		{result: commandOutput(repeat("C", 40))},
	}}
	result, err := Collect(context.Background(), runner, validRequest())
	if err != nil || result.Observation.HeadCommitSHA != repeat("a", 40) || result.Observation.BranchHint != "feature/wave-5" || result.Observation.Dirty == nil || !*result.Observation.Dirty || len(result.Observation.Files) != 1 || result.Observation.Files[0].CommitSHA != repeat("b", 40) || result.Observation.Files[0].BlobSHA != repeat("c", 40) || len(result.Failures) != 0 {
		t.Fatalf("Git metadata parse failed: %+v %v", result, err)
	}
	wantLast := []string{"/local/private/repository", "--no-optional-locks", "rev-parse", "HEAD:task-records/report.md"}
	if !reflect.DeepEqual(runner.calls[len(runner.calls)-1], wantLast) {
		t.Fatalf("unexpected argv: %v", runner.calls)
	}
	for _, call := range runner.calls {
		if len(call) < 2 || call[1] != "--no-optional-locks" {
			t.Fatalf("Git command may take optional locks: %v", call)
		}
	}
}

func TestCollectSupportsDetachedHeadAndCleanTree(t *testing.T) {
	runner := &fakeRunner{responses: []fakeResponse{
		{result: commandOutput(repeat("a", 64))},
		{result: CommandResult{ExitCode: 1}},
		{result: CommandResult{}},
	}}
	request := validRequest()
	request.RelativePaths = nil
	result, err := Collect(context.Background(), runner, request)
	if err != nil || result.Observation.BranchHint != "" || result.Observation.Dirty == nil || *result.Observation.Dirty || len(result.Failures) != 0 {
		t.Fatalf("detached/clean state failed: %+v %v", result, err)
	}
}

func TestCollectPreservesPartialInformationWithoutLeakingCommandErrors(t *testing.T) {
	runner := &fakeRunner{responses: []fakeResponse{
		{result: CommandResult{Stderr: "fatal: /local/private/repository missing", ExitCode: 128}, err: errors.New("exec failed")},
		{result: commandOutput("main")},
		{result: CommandResult{}},
		{result: CommandResult{ExitCode: 128}},
		{result: commandOutput(repeat("c", 40))},
	}}
	result, err := Collect(context.Background(), runner, validRequest())
	if err != nil || result.Observation.BranchHint != "main" || result.Observation.Files[0].BlobSHA == "" || len(result.Failures) != 2 {
		t.Fatalf("partial metadata discarded: %+v %v", result, err)
	}
	encoded, marshalErr := json.Marshal(result)
	if marshalErr != nil || strings.Contains(string(encoded), "/local/private/repository") || strings.Contains(string(encoded), "exec failed") {
		t.Fatalf("local path/error leaked into payload: %s %v", encoded, marshalErr)
	}
	requestPayload, marshalErr := json.Marshal(validRequest())
	if marshalErr != nil || strings.Contains(string(requestPayload), "/local/private/repository") {
		t.Fatalf("local path leaked from request DTO: %s %v", requestPayload, marshalErr)
	}
}

func TestCollectRejectsUnsafeOrNonCanonicalPaths(t *testing.T) {
	for _, relativePath := range []string{"/tmp/report.md", "../report.md", "task-records/./report.md", `C:\report.md`, "file:report.md"} {
		request := validRequest()
		request.RelativePaths = []string{relativePath}
		if _, err := Collect(context.Background(), &fakeRunner{}, request); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("unsafe path %q accepted: %v", relativePath, err)
		}
	}
	request := validRequest()
	request.WorktreeLabel = "/local/private/repository"
	if _, err := Collect(context.Background(), &fakeRunner{}, request); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal("absolute worktree label accepted")
	}
	for _, label := range []string{"review /Users/person/private/repo", "review/Users/person", "../review", " review"} {
		request := validRequest()
		request.WorktreeLabel = label
		if _, err := Collect(context.Background(), &fakeRunner{}, request); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("path-like worktree label %q accepted", label)
		}
	}
}

func validRequest() CollectRequest {
	return CollectRequest{
		RepositoryID: "repo", LocalRepositoryPath: "/local/private/repository", WorktreeLabel: "wave-5",
		RelativePaths: []string{"task-records/report.md"}, ObservedAt: time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC),
	}
}

func commandOutput(value string) CommandResult { return CommandResult{Stdout: value + "\n"} }

func repeat(value string, count int) string { return strings.Repeat(value, count) }
