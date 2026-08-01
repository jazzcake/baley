package runtimeconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDirectAndFileValues(t *testing.T) {
	t.Run("direct", func(t *testing.T) {
		t.Setenv("BALEY_TEST_VALUE", "direct-value")
		t.Setenv("BALEY_TEST_VALUE_FILE", "")
		value, configured, err := Load("BALEY_TEST_VALUE")
		if err != nil || !configured || value != "direct-value" {
			t.Fatalf("Load() = %q, %v, %v", value, configured, err)
		}
	})

	t.Run("file trims line ending only", func(t *testing.T) {
		t.Setenv("BALEY_TEST_VALUE", "")
		path := filepath.Join(t.TempDir(), "secret")
		if err := os.WriteFile(path, []byte("  file value  \r\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("BALEY_TEST_VALUE_FILE", path)
		value, configured, err := Load("BALEY_TEST_VALUE")
		if err != nil || !configured || value != "  file value  " {
			t.Fatalf("Load() = %q, %v, %v", value, configured, err)
		}
	})
}

func TestLoadRejectsAmbiguousOrInvalidFiles(t *testing.T) {
	t.Setenv("BALEY_TEST_VALUE", "direct")
	t.Setenv("BALEY_TEST_VALUE_FILE", filepath.Join(t.TempDir(), "secret"))
	if _, _, err := Load("BALEY_TEST_VALUE"); err == nil || !strings.Contains(err.Error(), "cannot both") {
		t.Fatalf("ambiguous sources accepted: %v", err)
	}

	t.Setenv("BALEY_TEST_VALUE", "")
	path := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BALEY_TEST_VALUE_FILE", path)
	if _, _, err := Load("BALEY_TEST_VALUE"); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("empty secret accepted: %v", err)
	}
}
