package doctor

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// The strip drops only [ui] advanced, whatever its spacing, and leaves a
// same-named key in another table alone.
func TestStripHubAdvanced(t *testing.T) {
	raw := []byte("[ui]\n  section = \"displays\"\n  advanced = \"1\"\n  update_system = \"off\"\n\n[other]\n  advanced=\"keep\"\n")
	want := []byte("[ui]\n  section = \"displays\"\n  update_system = \"off\"\n\n[other]\n  advanced=\"keep\"\n")
	out, changed := stripHubAdvanced(raw)
	if !changed || !bytes.Equal(out, want) {
		t.Fatalf("changed=%v out=\n%s\nwant\n%s", changed, out, want)
	}
	if _, changed := stripHubAdvanced(out); changed {
		t.Error("re-stripping a clean hub.toml must be a no-op")
	}
	if _, changed := stripHubAdvanced([]byte("[ui]\n  advanced_mode = \"x\"\n")); changed {
		t.Error("a key that only starts with advanced must stay")
	}
}

// Check-only reports and leaves bytes alone; the fix drops the line; a second
// run is clean.
func TestReconcileRetiredHubAdvanced(t *testing.T) {
	cfg := retiredKeysHome(t)
	path := filepath.Join(cfg, "ryoku", "hub.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := []byte("[ui]\n  section = \"displays\"\n  advanced = \"1\"\n")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	if r := reconcileRetiredHubAdvanced(true); r.status != recWouldFix {
		t.Fatalf("check-only = %s %q, want todo", r.status.label(), r.detail)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, raw) {
		t.Fatalf("check-only rewrote hub.toml:\n%s", got)
	}
	if r := reconcileRetiredHubAdvanced(false); r.status != recFixed {
		t.Fatalf("fix = %s %q, want fixed", r.status.label(), r.detail)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, []byte("[ui]\n  section = \"displays\"\n")) {
		t.Fatalf("fix left:\n%s", got)
	}
	if r := reconcileRetiredHubAdvanced(false); r.status != recOK {
		t.Fatalf("second run = %s %q, want ok", r.status.label(), r.detail)
	}
}
