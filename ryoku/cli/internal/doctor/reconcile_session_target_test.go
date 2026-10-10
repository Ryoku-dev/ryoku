package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-cli/internal/host"
)

func TestReconcileSessionTargetMigratesStaleWants(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	oldInit := sessionTargetInit
	sessionTargetInit = func() (host.InitSystem, error) { return host.Systemd, nil }
	t.Cleanup(func() { sessionTargetInit = oldInit })

	wants := filepath.Join(home, ".config", "systemd", "user", retiredSessionTarget+".wants")
	if err := os.MkdirAll(wants, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, u := range []string{"ryoku-shell.service", "ryogami.service"} {
		if err := os.WriteFile(filepath.Join(wants, u), []byte("stale symlink stand-in\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var enabled []string
	origEnable := enableUserUnit
	enableUserUnit = func(unit string) error { enabled = append(enabled, unit); return nil }
	defer func() { enableUserUnit = origEnable }()
	reloaded := false
	origReload := reloadUserDaemon
	reloadUserDaemon = func() { reloaded = true }
	defer func() { reloadUserDaemon = origReload }()

	if r := reconcileSessionTarget(true); r.status != recWouldFix {
		t.Fatalf("check: status=%s detail=%q, want a migration todo", r.status.label(), r.detail)
	}
	if _, err := os.Stat(wants); err != nil {
		t.Fatal("check-only must not touch the stale wants dir")
	}

	if r := reconcileSessionTarget(false); r.status != recFixed {
		t.Fatalf("repair: status=%s detail=%q, want fixed", r.status.label(), r.detail)
	}
	if len(enabled) != 2 {
		t.Fatalf("both units must be re-enabled, got %v", enabled)
	}
	if !reloaded {
		t.Fatal("repair must daemon-reload so systemd sees the relocated symlinks")
	}
	if _, err := os.Stat(wants); !os.IsNotExist(err) {
		t.Fatal("the retired wants dir must be gone after migration")
	}

	if r := reconcileSessionTarget(true); r.status != recOK {
		t.Fatalf("idempotent second run: status=%s, want ok", r.status.label())
	}
}

func TestReconcileSessionTargetUsesRunitRoster(t *testing.T) {
	oldInit := sessionTargetInit
	sessionTargetInit = func() (host.InitSystem, error) { return host.Runit, nil }
	t.Cleanup(func() { sessionTargetInit = oldInit })

	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	wants := filepath.Join(home, ".config", "systemd", "user", retiredSessionTarget+".wants")
	if err := os.MkdirAll(wants, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wants, "ryoku-shell.service"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	r := reconcileSessionTarget(false)
	if r.status != recOK || !strings.Contains(r.detail, "runit session roster") {
		t.Fatalf("runit result = %s %q, want neutral roster result", r.status.label(), r.detail)
	}
	if _, err := os.Stat(wants); err != nil {
		t.Fatal("runit check must not alter retired systemd state")
	}
}
