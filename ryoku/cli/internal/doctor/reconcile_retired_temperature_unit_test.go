package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

// general.temperature_unit leaves shell.json, the rest of general and the store
// stay, and a second run is a no-op.
func TestReconcileRetiredTemperatureUnit(t *testing.T) {
	dir := perfTestHome(t)
	path := filepath.Join(dir, "shell.json")

	if r := reconcileRetiredTemperatureUnit(false); r.status != recOK {
		t.Fatalf("missing shell.json: status=%s detail=%q, want ok", r.status.label(), r.detail)
	}
	stored := `{"general":{"clock_format_24_h":true,"temperature_unit":"Imperial"},"weatherUnit":"auto"}`
	writeTestStore(t, path, stored)

	if r := reconcileRetiredTemperatureUnit(true); r.status != recWouldFix {
		t.Fatalf("check: status=%s detail=%q, want would-fix", r.status.label(), r.detail)
	}
	if got, _ := os.ReadFile(path); string(got) != stored {
		t.Fatalf("check-only mutated shell.json: %s", got)
	}
	if r := reconcileRetiredTemperatureUnit(false); r.status != recFixed {
		t.Fatalf("fix: status=%s detail=%q, want fixed", r.status.label(), r.detail)
	}
	s := readTestStore(t, path)
	if dig(s, "general", "temperature_unit") != nil {
		t.Errorf("general.temperature_unit survived: %v", s)
	}
	if dig(s, "general", "clock_format_24_h") != true || s["weatherUnit"] != "auto" {
		t.Errorf("shell.json lost unrelated keys: %v", s)
	}
	if r := reconcileRetiredTemperatureUnit(false); r.status != recOK {
		t.Fatalf("clean store: status=%s detail=%q, want ok", r.status.label(), r.detail)
	}
}
