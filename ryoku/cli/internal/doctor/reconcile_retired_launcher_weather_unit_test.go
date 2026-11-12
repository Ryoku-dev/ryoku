package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

// A launcher unit moves into shell.json while the desktop's is Auto, then leaves
// launcher.json with the rest of the launcher kept; a second run is a no-op.
func TestReconcileRetiredLauncherWeatherUnit(t *testing.T) {
	dir := perfTestHome(t)
	launcherPath := filepath.Join(dir, "launcher.json")
	shellPath := filepath.Join(dir, "shell.json")

	if r := reconcileRetiredLauncherWeatherUnit(false); r.status != recOK {
		t.Fatalf("missing launcher.json: status=%s detail=%q, want ok", r.status.label(), r.detail)
	}

	launcher := `{"variant":"hero","weatherUnit":"F","radius":16}`
	writeTestStore(t, launcherPath, launcher)
	writeTestStore(t, shellPath, `{"weatherUnit":"auto","weatherLocation":"Oslo"}`)

	if r := reconcileRetiredLauncherWeatherUnit(true); r.status != recWouldFix {
		t.Fatalf("check: status=%s detail=%q, want would-fix", r.status.label(), r.detail)
	}
	if got, _ := os.ReadFile(launcherPath); string(got) != launcher {
		t.Fatalf("check-only mutated launcher.json: %s", got)
	}
	if r := reconcileRetiredLauncherWeatherUnit(false); r.status != recFixed {
		t.Fatalf("fix: status=%s detail=%q, want fixed", r.status.label(), r.detail)
	}
	l := readTestStore(t, launcherPath)
	if _, ok := l["weatherUnit"]; ok || l["variant"] != "hero" || l["radius"] != 16.0 {
		t.Errorf("launcher.json not stripped cleanly: %v", l)
	}
	s := readTestStore(t, shellPath)
	if s["weatherUnit"] != "fahrenheit" || s["weatherLocation"] != "Oslo" {
		t.Errorf("unit not carried into shell.json: %v", s)
	}
	if r := reconcileRetiredLauncherWeatherUnit(false); r.status != recOK {
		t.Fatalf("clean store: status=%s detail=%q, want ok", r.status.label(), r.detail)
	}
}

// The desktop's own unit wins, and a launcher Auto carries nothing.
func TestReconcileRetiredLauncherWeatherUnitKeepsDesktopUnit(t *testing.T) {
	dir := perfTestHome(t)
	launcherPath := filepath.Join(dir, "launcher.json")
	shellPath := filepath.Join(dir, "shell.json")

	writeTestStore(t, launcherPath, `{"weatherUnit":"F"}`)
	shell := `{"weatherUnit":"celsius"}`
	writeTestStore(t, shellPath, shell)
	if r := reconcileRetiredLauncherWeatherUnit(false); r.status != recFixed {
		t.Fatalf("fix: status=%s detail=%q, want fixed", r.status.label(), r.detail)
	}
	if got, _ := os.ReadFile(shellPath); string(got) != shell {
		t.Errorf("the desktop's own unit was overwritten: %s", got)
	}

	writeTestStore(t, launcherPath, `{"weatherUnit":"auto"}`)
	writeTestStore(t, shellPath, `{}`)
	if r := reconcileRetiredLauncherWeatherUnit(false); r.status != recFixed {
		t.Fatalf("fix auto: status=%s detail=%q, want fixed", r.status.label(), r.detail)
	}
	if got, _ := os.ReadFile(shellPath); string(got) != `{}` {
		t.Errorf("a launcher Auto carried something: %s", got)
	}
}
