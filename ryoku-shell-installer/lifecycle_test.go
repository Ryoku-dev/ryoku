package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunStateRoundtrip(t *testing.T) {
	home := t.TempDir()
	if loadState(home) != nil {
		t.Fatal("no file must mean no state")
	}
	e := &engine{f: &facts{homeDir: home}, p: &plan{}}
	e.markStepDone("legacy")
	e.backupDir = "/tmp/backup-x"
	e.markStepDone("backup")
	e.markStepDone("backup") // idempotent

	s := loadState(home)
	if s == nil || len(s.Completed) != 2 || !s.has("legacy") || !s.has("backup") || s.has("verify") {
		t.Fatalf("state wrong: %+v", s)
	}
	if s.BackupDir != "/tmp/backup-x" {
		t.Fatalf("backup dir not recorded: %+v", s)
	}

	e.clearState()
	if loadState(home) != nil {
		t.Fatal("clearState must remove the resume file")
	}
}

func TestRunStateIgnoresGarbage(t *testing.T) {
	home := t.TempDir()
	p := statePath(home)
	os.MkdirAll(home+"/.local/state/ryoku", 0o755)
	os.WriteFile(p, []byte("not json"), 0o644)
	if loadState(home) != nil {
		t.Fatal("garbage state must be ignored")
	}
	os.WriteFile(p, []byte(`{"completed":[]}`), 0o644)
	if loadState(home) != nil {
		t.Fatal("empty completed list is not a resumable run")
	}
}

func TestDryRunWritesNoState(t *testing.T) {
	home := t.TempDir()
	e := &engine{f: &facts{homeDir: home}, p: &plan{}, dry: true}
	e.markStepDone("legacy")
	if loadState(home) != nil {
		t.Fatal("dry runs must not write state")
	}
}

// A resume skips the payload step, so every later step must still find the
// checkout by absolute path: the session step joined an empty payload into
// "bash ryoku/lockscreen/sddm/setup", relative to wherever the installer was
// invoked from, and failed with 127 on every resumed run.
func TestNewEngineAnchorsPayloadOnResume(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", "")
	f := &facts{homeDir: home, prevRun: &runState{Completed: []string{"payload"}}}
	e := newEngine(f, defaultPlan(f), true, "main", "")
	want := filepath.Join(home, ".cache", "ryoku-shell-install", "repo")
	if e.payload != want {
		t.Fatalf("resume payload = %q, want %q", e.payload, want)
	}
	// The override wins over the cache path.
	e2 := newEngine(f, defaultPlan(f), true, "main", "/srv/checkout")
	if e2.payload != "/srv/checkout" {
		t.Fatalf("override payload = %q, want /srv/checkout", e2.payload)
	}
}

// A stable run that stopped after installing stable packages, resumed by the
// unstable script: skipping payload/repo/packages would lay unstable-dev
// configs over stable packages on the stable channel.
func TestResumeFromOtherRefRedoesRefBoundSteps(t *testing.T) {
	done := []string{"legacy", "sysupgrade", "tools", "payload", "backup", "repo", "conflicts", "packages", "drivers"}
	f := &facts{homeDir: t.TempDir(), prevRun: &runState{Completed: append([]string(nil), done...)}}
	p := defaultPlan(f)
	e := newEngine(f, p, true, "unstable-dev", "")
	for _, id := range []string{"payload", "repo", "packages"} {
		if e.state.has(id) {
			t.Errorf("step %s from the main run was kept for an unstable-dev resume", id)
		}
	}
	for _, id := range []string{"legacy", "backup", "conflicts", "drivers"} {
		if !e.state.has(id) {
			t.Errorf("ref-independent step %s was dropped from the resume", id)
		}
	}

	same := &facts{homeDir: t.TempDir(), prevRun: &runState{Completed: append([]string(nil), done...), Ref: "unstable-dev"}}
	if e := newEngine(same, defaultPlan(same), true, "unstable-dev", ""); len(e.state.Completed) != len(done) {
		t.Fatalf("a same-ref resume redid finished steps: %v", e.state.Completed)
	}
}

func TestVoidResumeCutsOverOldSourceState(t *testing.T) {
	done := []string{
		"sysupgrade", "tools", "payload", "backup", "conflicts", "packages",
		"fonts", "build", "drivers", "session", "configs", "shell", "doctor",
	}
	state := &runState{Completed: append([]string(nil), done...), Ref: "main", Delivery: "source"}
	f := &facts{homeDir: t.TempDir(), distro: voidLinux, prevRun: state}
	e := newEngine(f, defaultPlan(f), true, "main", "")
	for _, id := range []string{"payload", "packages", "build", "drivers", "session", "configs", "doctor"} {
		if e.state.has(id) {
			t.Errorf("source-lane step %s survived the packaged Void cutover", id)
		}
	}
	for _, id := range []string{"sysupgrade", "tools", "backup", "conflicts", "shell"} {
		if !e.state.has(id) {
			t.Errorf("delivery-independent step %s was dropped", id)
		}
	}
	if e.state.Delivery != "packages" {
		t.Fatalf("delivery = %q, want packages", e.state.Delivery)
	}

	current := &runState{Completed: append([]string(nil), done...), Ref: "main", Delivery: "packages"}
	f.prevRun = current
	if resumed := newEngine(f, defaultPlan(f), true, "main", ""); len(resumed.state.Completed) != len(done) {
		t.Fatalf("packaged Void resume redid finished steps: %v", resumed.state.Completed)
	}
}

func TestSetRyokuServerTouchesOnlyRyokuStanza(t *testing.T) {
	conf := "[options]\nArchitecture = auto\n\n[core]\nInclude = /etc/pacman.d/mirrorlist\n\n" +
		"[ryoku]\nSigLevel = Required\n#Server = https://old.example/$arch\nServer = " + stableServer + "\n\n" +
		"[custom]\nServer = https://mirror.example/$arch\n"
	got, prev := setRyokuServer(conf, testingServer)
	if prev != stableServer {
		t.Fatalf("replaced server = %q, want %q", prev, stableServer)
	}
	want := strings.Replace(conf, "Server = "+stableServer, "Server = "+testingServer, 1)
	if got != want {
		t.Fatalf("rewritten config:\n%s\nwant:\n%s", got, want)
	}
	if _, prev := setRyokuServer("[core]\nServer = x\n", testingServer); prev != "" {
		t.Fatalf("a config with no [ryoku] stanza reported server %q", prev)
	}
}
