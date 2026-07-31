package projectinit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyCreatesManifestAndRerunKeepsEverything(t *testing.T) {
	root := t.TempDir()
	input := validInput()
	input.BootstrapCompleted = true
	plan, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if err = Apply(root, plan); err != nil {
		t.Fatal(err)
	}
	input.ExistingFiles, err = LoadExistingFiles(root, input.TaskRecordsRoot)
	if err != nil {
		t.Fatal(err)
	}
	rerun, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range rerun.Files {
		if file.Action != FileKeep {
			t.Fatalf("rerun planned mutation: %+v", file)
		}
	}
}

func TestApplyRejectsStaleMergeBeforeWritingCreates(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".rgignore"), []byte("node_modules/**"), 0o600); err != nil {
		t.Fatal(err)
	}
	input := validInput()
	input.BootstrapCompleted = true
	input.ExistingFiles = map[string]string{".rgignore": "node_modules/**"}
	plan, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, ".rgignore"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = Apply(root, plan); err != ErrInvalidInput {
		t.Fatalf("stale merge accepted: %v", err)
	}
	if _, err = os.Stat(filepath.Join(root, "baley.yaml")); !os.IsNotExist(err) {
		t.Fatal("create occurred before stale merge rejection")
	}
}

func TestLoadExistingFilesRejectsSymlinkedManifestFile(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "baley.yaml")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := LoadExistingFiles(root, "task-records"); err != ErrInvalidInput {
		t.Fatalf("symlinked manifest file accepted: %v", err)
	}
}
