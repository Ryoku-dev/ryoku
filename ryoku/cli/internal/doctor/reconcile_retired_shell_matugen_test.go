package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

// theme.matugen leaves shell.json, the rest of theme and the store stay, and a
// second run is a no-op.
func TestReconcileRetiredShellMatugen(t *testing.T) {
	dir := perfTestHome(t)
	path := filepath.Join(dir, "shell.json")

	if r := reconcileRetiredShellMatugen(false); r.status != recOK {
		t.Fatalf("missing shell.json: status=%s detail=%q, want ok", r.status.label(), r.detail)
	}
	stored := `{"theme":{"theme":"Default","matugen":{"contrast":0,"mode":"Dark","preference":"Darkness","scheme_type":"TonalSpot"},"motion":{"scale":1.5}},"barStyle":"orpheus"}`
	writeTestStore(t, path, stored)

	if r := reconcileRetiredShellMatugen(true); r.status != recWouldFix {
		t.Fatalf("check: status=%s detail=%q, want would-fix", r.status.label(), r.detail)
	}
	if got, _ := os.ReadFile(path); string(got) != stored {
		t.Fatalf("check-only mutated shell.json: %s", got)
	}
	if r := reconcileRetiredShellMatugen(false); r.status != recFixed {
		t.Fatalf("fix: status=%s detail=%q, want fixed", r.status.label(), r.detail)
	}
	s := readTestStore(t, path)
	if dig(s, "theme", "matugen") != nil {
		t.Errorf("theme.matugen survived: %v", s)
	}
	if dig(s, "theme", "theme") != "Default" || dig(s, "theme", "motion", "scale") != 1.5 || s["barStyle"] != "orpheus" {
		t.Errorf("shell.json lost unrelated keys: %v", s)
	}
	if r := reconcileRetiredShellMatugen(false); r.status != recOK {
		t.Fatalf("clean store: status=%s detail=%q, want ok", r.status.label(), r.detail)
	}
}
