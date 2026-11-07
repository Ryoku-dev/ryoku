package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupQmlImportPathComposesStoreModulesAfterExistingRoots(t *testing.T) {
	configHome := t.TempDir()
	stateHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_STATE_HOME", stateHome)
	t.Setenv("QML_IMPORT_PATH", "/prior/qml")
	t.Setenv("QML2_IMPORT_PATH", "/prior/qml2")
	previousShellDir := shellDir
	shellDir = ""
	t.Cleanup(func() { shellDir = previousShellDir })

	setupQmlImportPath()
	root := filepath.Join(configHome, "quickshell")
	store := filepath.Join(stateHome, "ryoku", "store", "qml")
	for _, name := range []string{"QML_IMPORT_PATH", "QML2_IMPORT_PATH"} {
		parts := strings.Split(os.Getenv(name), string(os.PathListSeparator))
		if len(parts) < 3 || parts[0] != root {
			t.Fatalf("%s = %q, want shell root first", name, os.Getenv(name))
		}
		storeAt := -1
		for i, part := range parts {
			if part == store {
				storeAt = i
				break
			}
		}
		if storeAt <= 0 || storeAt != len(parts)-2 {
			t.Fatalf("%s = %q, want Store modules after daemon roots and before inherited path", name, os.Getenv(name))
		}
	}
}
