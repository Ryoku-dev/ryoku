package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// refusingProvider stands in for a provider whose apply regenerates its config from
// the store and then refuses a store holding REFUSE; it returns the generated file
func refusingProvider(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	generated := filepath.Join(t.TempDir(), "generated.conf")
	script := `#!/bin/sh
case "$1" in
caps) echo '{"name":"refusing","supports":[]}' ;;
apply)
  if [ -f "$2" ]; then cat "$2" > "$FAKE_GENERATED"; else echo absent > "$FAKE_GENERATED"; fi
  if grep -q REFUSE "$2" 2>/dev/null; then echo 'provider refused the store' >&2; exit 3; fi
  echo '{"reloadNeeded":false}' ;;
*) exit 2 ;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "ryoku-wm-refusing"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("RYOKU_WM", "refusing")
	t.Setenv("FAKE_GENERATED", generated)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	wmOnce = sync.Once{}
	wmClient = nil
	t.Cleanup(func() {
		wmOnce = sync.Once{}
		wmClient = nil
	})
	return generated
}

func readOrAbsent(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return "<absent>"
	}
	return string(b)
}

// a refused save puts the store, the provider's config and the live draft back as they were
func TestRefusedDesktopSaveLeavesNoTrace(t *testing.T) {
	for name, original := range map[string]string{"existing": `{"desktop":{"gaps":4}}`, "absent": ""} {
		t.Run(name, func(t *testing.T) {
			generated := refusingProvider(t)
			if original != "" {
				if err := saveDesktop(original); err != nil {
					t.Fatal(err)
				}
			}
			if err := applyDesktop(); err != nil {
				t.Fatal(err)
			}
			store, config := readOrAbsent(desktopStorePath()), readOrAbsent(generated)
			draft := `{"pid":1,"draft":{"desktop":{"gaps":"REFUSE"}}}`
			if err := atomicWrite(desktopPreviewPath(), []byte(draft), 0o600); err != nil {
				t.Fatal(err)
			}

			if err := saveDesktop(`{"desktop":{"gaps":"REFUSE"}}`); err == nil || !strings.Contains(err.Error(), "refused") {
				t.Fatalf("refused save = %v, want the provider's error", err)
			}
			if got := readOrAbsent(desktopStorePath()); got != store {
				t.Errorf("store after a refused save = %q, want %q", got, store)
			}
			if got := readOrAbsent(generated); got != config {
				t.Errorf("provider config after a refused save = %q, want %q", got, config)
			}
			if got := readOrAbsent(desktopPreviewPath()); got != draft {
				t.Errorf("live draft after a refused save = %q, want %q", got, draft)
			}

			if err := saveDesktop(`{"desktop":{"gaps":8}}`); err != nil {
				t.Fatal(err)
			}
			if original == "" {
				return
			}
			if err := saveDesktop(original); err != nil {
				t.Fatal(err)
			}
			if got := readOrAbsent(desktopStorePath()); got != store {
				t.Errorf("store after undo = %q, want %q", got, store)
			}
		})
	}
}

// a refused rebind leaves desktop.json and the provider's config as they were
func TestRefusedRebindLeavesNoTrace(t *testing.T) {
	generated := refusingProvider(t)
	if err := saveDesktop(`{"desktop":{"keybindRebinds":{"SUPER, Q":"SUPER, W"}}}`); err != nil {
		t.Fatal(err)
	}
	store, config := readOrAbsent(desktopStorePath()), readOrAbsent(generated)
	if err := setRebind("SUPER, Q", "REFUSE"); err == nil {
		t.Fatal("a refused rebind reported success")
	}
	if got := readOrAbsent(desktopStorePath()); got != store {
		t.Errorf("store after a refused rebind = %q, want %q", got, store)
	}
	if got := readOrAbsent(generated); got != config {
		t.Errorf("provider config after a refused rebind = %q, want %q", got, config)
	}
	if err := setRebind("SUPER, Q", "SUPER, E"); err != nil {
		t.Fatal(err)
	}
}
