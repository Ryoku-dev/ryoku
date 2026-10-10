package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetNomarchyActiveWithoutAssets(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("RYOKU_NOMARCHY_SOURCE", filepath.Join(t.TempDir(), "missing"))
	oldSystem := nomarchySystemSource
	nomarchySystemSource = filepath.Join(t.TempDir(), "missing-system")
	t.Cleanup(func() { nomarchySystemSource = oldSystem })

	if err := setNomarchyActive(true); err == nil {
		t.Fatal("activation without assets succeeded")
	}
	if err := setNomarchyActive(false); err != nil {
		t.Fatalf("deactivation without assets = %v, want nil", err)
	}
}

func TestSetNomarchyInactiveIgnoresCleanupFailure(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("RYOKU_NOMARCHY_SOURCE", root)
	for _, name := range []string{"nomarchy-activate", "nomarchy-deactivate"} {
		body := "#!/bin/sh\nexit 0\n"
		if name == "nomarchy-deactivate" {
			body = "#!/bin/sh\necho cleanup-failed >&2\nexit 1\n"
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := setNomarchyActive(false); err != nil {
		t.Fatalf("failed cleanup blocked deactivation: %v", err)
	}
}
