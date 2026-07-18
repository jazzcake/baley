package domain

import "testing"

func TestNormalizeRecordPath(t *testing.T) {
	tests := []struct {
		name, root, input, want string
		invalid                 bool
	}{
		{name: "normal", root: "task-records", input: "task-records/task-104/report.md", want: "task-records/task-104/report.md"},
		{name: "cleans separators", root: "records/./", input: "records/task//리뷰.md", want: "records/task/리뷰.md"},
		{name: "encoded dots remain filename", root: "records", input: "records/%2e%2e/report.md", want: "records/%2e%2e/report.md"},
		{name: "absolute", root: "records", input: "/records/a.md", invalid: true},
		{name: "windows absolute", root: "records", input: `C:\records\a.md`, invalid: true},
		{name: "unc", root: "records", input: `\\server\share\a.md`, invalid: true},
		{name: "uri", root: "records", input: "file:///records/a.md", invalid: true},
		{name: "parent", root: "records", input: "records/../secret.md", invalid: true},
		{name: "outside sibling", root: "records", input: "records-old/a.md", invalid: true},
		{name: "root is not file", root: "records", input: "records", invalid: true},
		{name: "nul", root: "records", input: "records/a\x00.md", invalid: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeRecordPath(tt.root, tt.input)
			if tt.invalid {
				assertViolation(t, err, CodeInvalidRecordPath)
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}
