package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopRejectsNullDraft(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// No provider or session helper may run during a rejected save.
	t.Setenv("PATH", t.TempDir())
	const saved = `{"desktop":{"cursor":{"size":32}}}`
	for name, run := range map[string]func(string) error{"save": saveDesktop, "preview": previewDesktop} {
		t.Run(name, func(t *testing.T) {
			if err := atomicWrite(desktopStorePath(), []byte(saved), 0600); err != nil {
				t.Fatal(err)
			}
			if err := run("null"); err == nil || !strings.Contains(err.Error(), "JSON object") {
				t.Errorf("expected an invalid-draft error, got %v", err)
			}
			got, err := os.ReadFile(desktopStorePath())
			if err != nil || string(got) != saved {
				t.Errorf("rejected draft changed saved settings: %q, %v", got, err)
			}
		})
	}
}

func TestReadJSONMapCanEditNullStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")
	if err := os.WriteFile(path, []byte("null"), 0600); err != nil {
		t.Fatal(err)
	}
	store := readJSONMap(path)
	if store == nil {
		t.Fatal("null store returned a nil map; the next edit would panic")
	}
	childMap(store, "desktop")["cursor"] = map[string]any{"size": 32}
}
