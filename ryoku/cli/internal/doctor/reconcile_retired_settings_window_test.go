package doctor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	wm "ryoku-wm"
)

func retiredKeysHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("RYOKU_REPO", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	return filepath.Join(home, ".config")
}

// The strip drops settingsWindow from every provider object, keeps their other
// leaves and the desktop namespace, and is a no-op on a clean store.
func TestStripSettingsWindow(t *testing.T) {
	a, b := wm.ProviderNiri, wm.ProviderWayfire
	raw := []byte(fmt.Sprintf(`{"desktop":{"appearance":{"rounding":8}},"wm":{%q:{"settingsWindow":{"mode":"custom","width":0.5,"height":0.5},"defaultColumnWidth":0.5},%q:{"settingsWindow":{"mode":"full"}}}}`, a, b))
	out, changed, err := stripSettingsWindow(raw)
	if err != nil || !changed {
		t.Fatalf("settingsWindow must be stripped: changed=%v err=%v", changed, err)
	}
	var doc struct {
		Desktop map[string]any            `json:"desktop"`
		WM      map[string]map[string]any `json:"wm"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("stripped JSON does not parse: %v", err)
	}
	for name, prov := range doc.WM {
		if _, present := prov["settingsWindow"]; present {
			t.Errorf("wm.%s.settingsWindow survived the strip", name)
		}
	}
	if doc.WM[a]["defaultColumnWidth"] != 0.5 {
		t.Errorf("wm.%s.defaultColumnWidth was lost: %v", a, doc.WM[a])
	}
	if _, present := doc.WM[b]; !present {
		t.Errorf("wm.%s must stay even when the strip empties it", b)
	}
	if _, present := doc.Desktop["appearance"]; !present {
		t.Errorf("desktop namespace was lost: %v", doc.Desktop)
	}
	if _, changed, err := stripSettingsWindow(out); err != nil || changed {
		t.Errorf("re-stripping a clean store must be a no-op: changed=%v err=%v", changed, err)
	}
	if _, changed, err := stripSettingsWindow([]byte(`{"desktop":{}}`)); err != nil || changed {
		t.Errorf("a store with no wm namespace must be untouched: changed=%v err=%v", changed, err)
	}
}

// Check-only reports and leaves bytes alone; the fix strips the store and
// removes every retired generated module; a second run is clean.
func TestReconcileRetiredSettingsWindow(t *testing.T) {
	cfg := retiredKeysHome(t)
	store := filepath.Join(cfg, "ryoku", "desktop.json")
	if err := os.MkdirAll(filepath.Dir(store), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := []byte(fmt.Sprintf(`{"wm":{%q:{"settingsWindow":{"mode":"custom","width":0.5,"height":0.5},"plugins":{}}}}`, wm.ProviderHyprland))
	if err := os.WriteFile(store, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	var modules []string
	for _, name := range wm.Providers() {
		for _, rel := range wm.RetiredGeneratedConfig(name) {
			path := filepath.Join(cfg, rel)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("-- generated\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			modules = append(modules, path)
		}
	}
	if len(modules) == 0 {
		t.Fatal("the seam names no retired generated module")
	}

	if r := reconcileRetiredSettingsWindow(true); r.status != recWouldFix {
		t.Fatalf("check-only = %s %q, want todo", r.status.label(), r.detail)
	}
	if got, _ := os.ReadFile(store); !bytes.Equal(got, raw) {
		t.Fatalf("check-only rewrote the store:\n%s", got)
	}
	for _, path := range modules {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("check-only removed %s", path)
		}
	}

	if r := reconcileRetiredSettingsWindow(false); r.status != recFixed {
		t.Fatalf("fix = %s %q, want fixed", r.status.label(), r.detail)
	}
	got, err := os.ReadFile(store)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got, []byte("settingsWindow")) {
		t.Fatalf("fix left settingsWindow in the store:\n%s", got)
	}
	if !bytes.Contains(got, []byte(`"plugins"`)) {
		t.Fatalf("fix dropped a sibling leaf:\n%s", got)
	}
	for _, path := range modules {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Errorf("fix left %s behind", path)
		}
	}

	if r := reconcileRetiredSettingsWindow(false); r.status != recOK {
		t.Fatalf("second run = %s %q, want ok", r.status.label(), r.detail)
	}
}
