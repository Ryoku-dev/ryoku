package updater

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
)

// readRunState decodes the published run-state file.
func readRunState(t *testing.T) runState {
	t.Helper()
	b, err := os.ReadFile(runStatePath())
	if err != nil {
		t.Fatalf("read run-state: %v", err)
	}
	var st runState
	if err := json.Unmarshal(b, &st); err != nil {
		t.Fatalf("decode run-state: %v (%s)", err, b)
	}
	return st
}

// The step publisher drives a determinate multi-step run: begin lists the
// stages, at advances (marking earlier ones done), and progress tracks the
// step states so the GUI shows a real bar, not a fake wave.
func TestProgressStepsAdvance(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	p := &publisher{}
	p.begin(pkgSteps)
	t.Cleanup(p.idle)

	st := readRunState(t)
	if st.PID != os.Getpid() {
		t.Fatalf("begin: pid=%d, want %d", st.PID, os.Getpid())
	}
	if st.Phase != "running" || len(st.Steps) != len(pkgSteps) {
		t.Fatalf("begin: phase=%q steps=%d, want running/%d", st.Phase, len(st.Steps), len(pkgSteps))
	}
	for _, s := range st.Steps {
		if s.State != stepPending {
			t.Fatalf("begin: step %q state %q, want pending", s.Key, s.State)
		}
	}

	p.at("packages")
	st = readRunState(t)
	if st.Step != "packages" || st.Label == "" {
		t.Fatalf("at(packages): step=%q label=%q", st.Step, st.Label)
	}
	if st.Steps[0].State != stepDone { // snapshot marked done
		t.Errorf("at(packages): snapshot state %q, want ok", st.Steps[0].State)
	}
	if st.Steps[1].State != stepRunning { // packages running
		t.Errorf("at(packages): packages state %q, want running", st.Steps[1].State)
	}
	// progress is determinate: 1 done + 0.5 running over 7 steps.
	if st.Progress <= 0 || st.Progress >= 1 {
		t.Errorf("at(packages): progress %v, want strictly between 0 and 1", st.Progress)
	}
}
func TestProgressPublishesVoidSnapshotSupport(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("RYOKU_HOST_PKGMGR", "xbps")
	p := &publisher{}
	p.begin(pkgSteps)
	t.Cleanup(p.idle)

	st := readRunState(t)
	if !st.SnapshotsSupported {
		t.Fatal("XBPS run-state reports snapshots unsupported")
	}
	if st.SnapshotReason != "" {
		t.Fatalf("XBPS run-state snapshot reason = %q, want empty", st.SnapshotReason)
	}
}

// A logged line lands in the rolling ring, capped, and republishes.
func TestProgressLogRing(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	p := &publisher{}
	p.begin(pkgSteps)
	t.Cleanup(p.idle)
	for i := range runLogCap + 5 {
		p.logf("line %d", i)
	}
	st := readRunState(t)
	if len(st.Log) != runLogCap {
		t.Fatalf("log ring len %d, want capped at %d", len(st.Log), runLogCap)
	}
	if st.Log[len(st.Log)-1] != "line 16" {
		t.Errorf("log tail = %q, want newest line kept", st.Log[len(st.Log)-1])
	}
}

// fail records the failed step, the reason, and preserves the pre-update
// snapshot so the GUI can offer a one-click rollback.
func TestProgressFailCarriesSnapshotAndError(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	p := &publisher{}
	p.begin(pkgSteps)
	p.setSnapshot("42")
	p.at("packages")
	p.fail(errors.New("pacman blew up"))

	st := readRunState(t)
	if st.Phase != "error" {
		t.Fatalf("fail: phase %q, want error", st.Phase)
	}
	if st.Error != "pacman blew up" {
		t.Errorf("fail: error %q", st.Error)
	}
	if st.Snapshot != "42" {
		t.Errorf("fail: snapshot %q, want 42 for rollback", st.Snapshot)
	}
	if st.Steps[1].State != stepFailed {
		t.Errorf("fail: packages state %q, want failed", st.Steps[1].State)
	}
}

// finish marks a terminal done state with full progress; idle clears it so the
// island folds.
func TestProgressFinishThenIdle(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	p := &publisher{}
	p.begin(gitSteps)
	p.at("deploy")
	p.finish()
	if st := readRunState(t); st.Phase != "done" || st.Progress != 1 {
		t.Fatalf("finish: phase=%q progress=%v, want done/1", st.Phase, st.Progress)
	}
	p.idle()
	if st := readRunState(t); st.Phase != "idle" {
		t.Fatalf("idle: phase %q, want idle", st.Phase)
	}
}

// Narration outside a run (`ryoku track`, the channel helpers under test) must
// never write the run-state: a "running" document with no live owner is what
// pinned the Hub on a phantom update it could not leave.
func TestNarrationOutsideARunLeavesRunStateAlone(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	p := &publisher{}
	p.logf("Removed %s: the target channel does not serve it", "ryoku-desktop-hyprland")
	p.detailf("Fetching origin/main")
	p.observe("(1/3) upgrading ryoku")
	if _, err := os.Stat(runStatePath()); !os.IsNotExist(err) {
		t.Fatalf("narration outside a run wrote %s (err=%v)", runStatePath(), err)
	}
}

// The exec handoff keeps the pid, so stage2's begin picks up the run stage1
// published: its start time, its narrative, and the steps already behind it,
// with the step stage1 was on when it handed over counted as done.
func TestStage2AdoptsTheRunStage1Published(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	stage1 := runState{
		Phase: "running", PID: os.Getpid(), Started: 1000, Log: []string{"Updating 3 Ryoku package(s)"},
		Steps: []runStep{
			{Key: "snapshot", State: stepDone, Began: 1000, Took: 800},
			{Key: "packages", State: stepRunning, Began: 2000},
		},
	}
	writeState(stage1)
	p := &publisher{}
	resumed := p.begin(pkgSteps)
	st := readRunState(t)
	p.idle()
	if !resumed {
		t.Fatal("begin did not resume the run stage1 published under the same pid")
	}
	if st.Started != 1000 || len(st.Log) != 1 {
		t.Errorf("resumed run lost stage1's start or narrative: started=%d log=%q", st.Started, st.Log)
	}
	if st.Steps[0].State != stepDone || st.Steps[0].Took != 800 {
		t.Errorf("snapshot step = %+v, want done with stage1's timing", st.Steps[0])
	}
	if st.Steps[1].State != stepDone {
		t.Errorf("packages step = %q, want done: stage1 only hands over after it succeeds", st.Steps[1].State)
	}

	// another process's run is never adopted
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	stage1.PID = os.Getpid() + 1
	writeState(stage1)
	q := &publisher{}
	if q.begin(pkgSteps) {
		t.Error("begin resumed a run published by a different pid")
	}
	t.Cleanup(q.idle)
}
