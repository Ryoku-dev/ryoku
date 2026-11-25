package task_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/hands/discipline"
	"ryoku-rashin/internal/oni/hands/loop"
	"ryoku-rashin/internal/oni/hands/proto"
	"ryoku-rashin/internal/oni/hands/task"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
)

// Every reason the loop ends or pauses a task with, and every pause the
// window discipline hands the loop, is kept and reads back; nothing else
// is, not even the discipline's pick refusals.
func TestTheReasonsAreExactlyTheLoopsAndTheDisciplines(t *testing.T) {
	ends := []string{
		loop.EndDone, loop.EndAsked, loop.EndAnswered, loop.EndStopped, loop.EndBudget,
		loop.EndRepeat, loop.EndUnconfirmed, loop.EndRefused, loop.EndExpired, loop.EndAway,
		loop.EndTimeout, loop.EndFailed, loop.EndClosed, loop.EndUnread,
	}
	pauses := []string{loop.PauseClosed, loop.PauseNavigation, loop.PauseWindow, loop.PauseFocus}
	for _, p := range []discipline.Pause{
		discipline.PauseClosed, discipline.PauseFocus, discipline.PauseWindow, discipline.PauseHidden,
		discipline.PauseOutput, discipline.PauseTitle, discipline.PauseGeometry, discipline.PauseCovered,
		discipline.PauseSelf, discipline.PausePrivate, discipline.PauseKey, discipline.PausePresenter,
	} {
		pauses = append(pauses, string(p))
	}
	j := open(t)
	for i, why := range pauses {
		a := enqueue(t, j, window("org.example.Editor"), t0.Add(time.Duration(i)*time.Minute))
		update(t, j, a.ID, task.NeedsYou, 1, why, t0.Add(time.Duration(i)*time.Minute))
		update(t, j, a.ID, task.Stopped, 1, loop.EndStopped, t0.Add(time.Duration(i)*time.Minute))
	}
	for i, why := range ends {
		at := t0.Add(time.Hour + time.Duration(i)*time.Minute)
		a := enqueue(t, j, window("org.example.Editor"), at)
		update(t, j, a.ID, task.Failed, 1, why, at)
	}
	ts, err := task.Open(j.Path()).List()
	if err != nil || len(ts) != len(pauses)+len(ends) {
		t.Fatalf("journal read back %d rows, %v; want %d", len(ts), err, len(pauses)+len(ends))
	}

	a := enqueue(t, j, window("org.example.Editor"), t0.Add(2*time.Hour))
	refused := []string{
		string(discipline.ReasonMissing), string(discipline.ReasonUnpinned), "handoff", "paused",
		"Focus", "focus ", "navigation?", loop.ByButton,
	}
	for _, why := range refused {
		if _, err := j.Update(a.ID, task.NeedsYou, 1, why, t0.Add(2*time.Hour)); reason(err) != protocol.CodeBadRequest {
			t.Errorf("reason %q = %v, want bad_request", why, err)
		}
	}
}

var t0 = time.Date(2026, 11, 23, 9, 0, 0, 0, time.UTC)

func open(t *testing.T) *task.Journal {
	t.Helper()
	return task.Open(filepath.Join(t.TempDir(), "state", "hyakki", "oni-hands-tasks.json"))
}

func reason(err error) string {
	var pe *protocol.Error
	if errors.As(err, &pe) {
		if pe.Reason != "" {
			return pe.Reason
		}
		return pe.Code
	}
	if err == nil {
		return ""
	}
	return "other: " + err.Error()
}

func window(app string) task.Task {
	return task.Task{ID: task.NewID(), Scope: proto.ScopeWindow, App: app, Origin: task.OriginApp}
}

func enqueue(t *testing.T, j *task.Journal, tk task.Task, at time.Time) task.Task {
	t.Helper()
	got, err := j.Enqueue(tk, at)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	return got
}

func update(t *testing.T, j *task.Journal, id string, st task.State, step int, why string, at time.Time) task.Task {
	t.Helper()
	got, err := j.Update(id, st, step, why, at)
	if err != nil {
		t.Fatalf("Update %s: %v", st, err)
	}
	return got
}

func TestSpecNumbers(t *testing.T) {
	s, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	c := task.FromSpec(s)
	if c.QueueMax != s.Hands.Task.QueueMax || c.Turns != s.Hands.Task.Turns || c.Rows != s.Hands.Retention.Rows || c.Age != s.Hands.Retention.Age() {
		t.Fatalf("FromSpec = %+v, not the spec's hands.task and hands.retention", c)
	}
	if c.QueueMax < 1 || c.Rows <= c.QueueMax || c.Age <= 0 || c.Turns < 1 {
		t.Fatalf("spec numbers unusable: %+v", c)
	}
}

func TestQueueIsFIFOAndOnlyAButtonStartsAWaitingTask(t *testing.T) {
	j := open(t)
	a := enqueue(t, j, window("org.example.Editor"), t0)
	if a.State != task.Working {
		t.Fatalf("first task = %s, want it to start by its own claim", a.State)
	}
	b := enqueue(t, j, window("org.example.Editor"), t0.Add(time.Second))
	if b.State != task.UpNext {
		t.Fatalf("second task = %s, want it to wait", b.State)
	}
	if _, err := j.Enqueue(window("org.example.Editor"), t0.Add(2*time.Second)); reason(err) != task.ReasonFull {
		t.Fatalf("a task past the queue = %v, want %s", err, task.ReasonFull)
	}
	if _, err := j.Start(b.ID, task.ByButton, t0.Add(3*time.Second)); reason(err) != task.ReasonBusy {
		t.Fatalf("start while a task runs = %v, want %s", err, task.ReasonBusy)
	}
	update(t, j, a.ID, task.Done, 3, loop.EndDone, t0.Add(4*time.Second))
	if _, ok, err := j.Running(); ok || err != nil {
		t.Fatalf("Running after the first ended = %v, %v; the waiting task must not start by itself", ok, err)
	}
	for _, by := range []string{"", "press", "voice", "wake", "cli", "harness", "Button"} {
		if _, err := j.Start(b.ID, by, t0.Add(5*time.Second)); reason(err) != protocol.CodeForbidden {
			t.Fatalf("Start by %q = %v, want forbidden", by, err)
		}
	}
	got, err := j.Start(b.ID, task.ByButton, t0.Add(5*time.Second))
	if err != nil || got.State != task.Working {
		t.Fatalf("Start by button = %+v, %v", got, err)
	}
	if run, ok, _ := j.Running(); !ok || run.ID != b.ID {
		t.Fatalf("Running = %+v, %v; want the started task", run, ok)
	}
}

func TestNewTaskWaitsBehindAWaitingOne(t *testing.T) {
	j := open(t)
	a := enqueue(t, j, window("org.example.Editor"), t0)
	b := enqueue(t, j, window("org.example.Editor"), t0.Add(time.Second))
	update(t, j, a.ID, task.Stopped, 0, loop.EndStopped, t0.Add(2*time.Second))
	// Nothing runs, but one waits: a new claim does not jump the queue.
	if _, err := j.Enqueue(window("org.example.Editor"), t0.Add(3*time.Second)); reason(err) != task.ReasonFull {
		t.Fatalf("a claim behind a waiting task = %v, want %s", err, task.ReasonFull)
	}
	// A waiting task may be called off without ever starting.
	got := update(t, j, b.ID, task.Stopped, 0, loop.EndStopped, t0.Add(4*time.Second))
	if got.Ended.IsZero() {
		t.Fatalf("a stopped waiting task has no end: %+v", got)
	}
	if c := enqueue(t, j, window("org.example.Editor"), t0.Add(5*time.Second)); c.State != task.Working {
		t.Fatalf("a claim with nothing running or waiting = %s, want working", c.State)
	}
}

func TestEndedTasksNeverStartAgain(t *testing.T) {
	for _, end := range []task.State{task.Done, task.Failed, task.Stopped} {
		t.Run(string(end), func(t *testing.T) {
			j := open(t)
			a := enqueue(t, j, window("org.example.Editor"), t0)
			update(t, j, a.ID, task.Checking, 1, "", t0.Add(time.Second))
			update(t, j, a.ID, end, 2, loop.EndFailed, t0.Add(2*time.Second))
			if _, err := j.Start(a.ID, task.ByButton, t0.Add(3*time.Second)); reason(err) != task.ReasonNotStartable {
				t.Fatalf("Start of a %s task = %v, want %s", end, err, task.ReasonNotStartable)
			}
			if _, err := j.Update(a.ID, task.Working, 2, "", t0.Add(3*time.Second)); reason(err) != task.ReasonNotStartable {
				t.Fatalf("Update of a %s task = %v, want %s", end, err, task.ReasonNotStartable)
			}
			ts, _ := j.List()
			if len(ts) != 1 || ts[0].State != end {
				t.Fatalf("journal = %+v", ts)
			}
		})
	}
}

func TestUpdateBounds(t *testing.T) {
	j := open(t)
	a := enqueue(t, j, window("org.example.Editor"), t0)
	update(t, j, a.ID, task.Working, 5, "", t0.Add(time.Second))
	cases := []struct {
		name string
		st   task.State
		step int
		why  string
	}{
		{"step back", task.Working, 4, ""},
		{"past the turns", task.Working, limits(t).Turns + 1, ""},
		{"to upnext", task.UpNext, 5, ""},
		{"to interrupted", task.Interrupted, 5, task.ReasonRestart},
		{"unknown state", task.State("paused"), 5, ""},
		{"free-text reason", task.NeedsYou, 5, "the window title changed"},
		{"word reason", task.NeedsYou, 5, "zebra"},
	}
	for _, c := range cases {
		if _, err := j.Update(a.ID, c.st, c.step, c.why, t0.Add(2*time.Second)); reason(err) != protocol.CodeBadRequest {
			t.Errorf("%s: Update = %v, want bad_request", c.name, err)
		}
	}
	got := update(t, j, a.ID, task.NeedsYou, 6, string(discipline.PauseFocus), t0.Add(3*time.Second))
	if got.Reason != string(discipline.PauseFocus) || !got.Ended.IsZero() {
		t.Fatalf("a pause = %+v", got)
	}
	// A clock set back never ends a task before it was queued.
	got = update(t, j, a.ID, task.Done, 6, loop.EndDone, t0.Add(-time.Hour))
	if got.Ended.Before(got.At) {
		t.Fatalf("ended %v before queued %v", got.Ended, got.At)
	}
	if _, err := task.Open(j.Path()).List(); err != nil {
		t.Fatalf("the journal does not read back: %v", err)
	}
}

func limits(t *testing.T) task.Config {
	t.Helper()
	s, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	return task.FromSpec(s)
}

func TestRestartInterruptsAndNothingResumes(t *testing.T) {
	j := open(t)
	a := enqueue(t, j, window("org.example.Editor"), t0)
	update(t, j, a.ID, task.Checking, 4, "", t0.Add(time.Second))
	b := enqueue(t, j, task.Task{ID: task.NewID(), Scope: proto.ScopeNested, Origin: task.OriginPresenter}, t0.Add(2*time.Second))

	// The daemon dies and comes back: a fresh journal on the same file.
	restarted := task.Open(j.Path())
	held, err := restarted.Recover(t0.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(held) != 2 {
		t.Fatalf("Recover held %d tasks, want the running and the waiting one: %+v", len(held), held)
	}
	for _, h := range held {
		if h.State != task.Interrupted || h.Reason != task.ReasonRestart || h.Ended.IsZero() {
			t.Fatalf("held task = %+v", h)
		}
	}
	if _, ok, err := restarted.Running(); ok || err != nil {
		t.Fatalf("a task runs after Recover: %v, %v", ok, err)
	}
	if again, err := restarted.Recover(t0.Add(2 * time.Hour)); err != nil || len(again) != 0 {
		t.Fatalf("a second Recover = %+v, %v; want nothing", again, err)
	}
	for _, id := range []string{a.ID, b.ID} {
		for _, by := range []string{"press", "voice", "wake", "cli", "harness"} {
			if _, err := restarted.Start(id, by, t0.Add(3*time.Hour)); reason(err) != protocol.CodeForbidden {
				t.Fatalf("Start of an interrupted task by %s = %v, want forbidden", by, err)
			}
		}
	}
	got, err := restarted.Start(a.ID, task.ByButton, t0.Add(3*time.Hour))
	if err != nil || got.State != task.Working || got.Step != 4 || got.Reason != "" || !got.Ended.IsZero() {
		t.Fatalf("Start of an interrupted task by button = %+v, %v; want it working at its old step", got, err)
	}
	if _, err := restarted.Start(b.ID, task.ByButton, t0.Add(3*time.Hour)); reason(err) != task.ReasonBusy {
		t.Fatalf("a second start = %v, want %s", err, task.ReasonBusy)
	}
}

func TestDamagedFileIsNeverOverwritten(t *testing.T) {
	cases := map[string]string{
		"garbage":       "{not json",
		"smuggled goal": `{"v":1,"tasks":[{"id":"0123456789abcdef0123456789abcdef","state":"done","scope":"nested","origin":"app","step":1,"at":"2026-11-23T09:00:00Z","ended":"2026-11-23T09:01:00Z","goal":"write to mum"}],"projects":[]}`,
		"title as app":  `{"v":1,"tasks":[{"id":"0123456789abcdef0123456789abcdef","state":"done","scope":"window","app":"Inbox - mum","origin":"app","step":1,"at":"2026-11-23T09:00:00Z","ended":"2026-11-23T09:01:00Z"}],"projects":[]}`,
		"two running":   `{"v":1,"tasks":[{"id":"0123456789abcdef0123456789abcdef","state":"working","scope":"nested","origin":"app","step":0,"at":"2026-11-23T09:00:00Z"},{"id":"1123456789abcdef0123456789abcdef","state":"checking","scope":"nested","origin":"app","step":0,"at":"2026-11-23T09:00:00Z"}],"projects":[]}`,
		"ended unset":   `{"v":1,"tasks":[{"id":"0123456789abcdef0123456789abcdef","state":"failed","scope":"nested","origin":"app","step":0,"at":"2026-11-23T09:00:00Z"}],"projects":[]}`,
		"wrong version": `{"v":2,"tasks":[],"projects":[]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			j := open(t)
			if err := os.MkdirAll(filepath.Dir(j.Path()), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(j.Path(), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			ops := map[string]func() error{
				"Enqueue":   func() error { _, err := j.Enqueue(window("org.example.Editor"), t0); return err },
				"Start":     func() error { _, err := j.Start(task.NewID(), task.ByButton, t0); return err },
				"Update":    func() error { _, err := j.Update(task.NewID(), task.Done, 0, "", t0); return err },
				"Recover":   func() error { _, err := j.Recover(t0); return err },
				"Purge":     func() error { return j.Purge(t0.Add(365 * 24 * time.Hour)) },
				"List":      func() error { _, err := j.List(); return err },
				"Forget":    func() error { return j.Forget(task.NewID()) },
				"ForgetAll": j.ForgetAll,
				"Keep":      func() error { _, err := j.Keep(task.NewID(), task.NewID(), t0); return err },
				"Continue": func() error {
					_, err := j.Continue(task.NewID(), task.NewID(), task.OriginApp, t0)
					return err
				},
			}
			for op, call := range ops {
				if err := call(); reason(err) != task.ReasonDamaged {
					t.Errorf("%s on a damaged journal = %v, want %s", op, err, task.ReasonDamaged)
				}
				if data, _ := os.ReadFile(j.Path()); string(data) != body {
					t.Fatalf("%s rewrote the damaged journal", op)
				}
			}
			if err := j.Reset(); err != nil {
				t.Fatal(err)
			}
			if data, _ := os.ReadFile(j.DamagedPath()); string(data) != body {
				t.Fatal("Reset did not keep the old file beside")
			}
			if got := enqueue(t, j, window("org.example.Editor"), t0); got.State != task.Working {
				t.Fatalf("after Reset = %+v", got)
			}
		})
	}
}

func TestRetentionByDaysAndRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oni-hands-tasks.json")
	j := task.OpenWith(path, task.Config{Rows: 3})
	var ids []string
	for i := range 5 {
		at := t0.Add(time.Duration(i) * time.Minute)
		a := enqueue(t, j, window("org.example.Editor"), at)
		update(t, j, a.ID, task.Done, 1, loop.EndDone, at.Add(time.Second))
		ids = append(ids, a.ID)
	}
	ts, err := j.List()
	if err != nil || len(ts) != 3 {
		t.Fatalf("rows kept = %d, %v; want 3", len(ts), err)
	}
	for i, tk := range ts {
		if tk.ID != ids[i+2] {
			t.Fatalf("kept %v, want the three latest", ts)
		}
	}

	day := 24 * time.Hour
	age := limits(t).Age
	j = task.Open(filepath.Join(t.TempDir(), "oni-hands-tasks.json"))
	old := enqueue(t, j, window("org.example.Editor"), t0)
	update(t, j, old.ID, task.Done, 1, loop.EndDone, t0)
	recent := enqueue(t, j, window("org.example.Editor"), t0.Add(2*day))
	update(t, j, recent.ID, task.Stopped, 1, loop.EndStopped, t0.Add(2*day))
	running := enqueue(t, j, window("org.example.Editor"), t0.Add(3*day))
	pOld, err := j.Keep(old.ID, task.NewID(), t0)
	if err != nil {
		t.Fatal(err)
	}
	pNew, err := j.Keep(recent.ID, task.NewID(), t0.Add(2*day))
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Purge(t0.Add(age + day)); err != nil {
		t.Fatal(err)
	}
	ts, _ = j.List()
	if len(ts) != 2 || ts[0].ID != recent.ID || ts[1].ID != running.ID {
		t.Fatalf("after Purge = %+v; want the recent and the running task", ts)
	}
	ps, _ := j.Projects()
	if len(ps) != 1 || ps[0].ID != pNew.ID {
		t.Fatalf("projects after Purge = %+v; want only %s, not %s", ps, pNew.ID, pOld.ID)
	}
	// Long after, a running task is still never purged.
	if err := j.Purge(t0.Add(10 * age)); err != nil {
		t.Fatal(err)
	}
	ts, _ = j.List()
	if len(ts) != 1 || ts[0].ID != running.ID {
		t.Fatalf("after a late Purge = %+v; want only the running task", ts)
	}
}

func TestDeleteAllAndNoFileAtRest(t *testing.T) {
	j := open(t)
	a := enqueue(t, j, window("org.example.Editor"), t0)
	info, err := os.Stat(j.Path())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("journal mode %v, want 0600", info.Mode().Perm())
	}
	if dir, _ := os.Stat(filepath.Dir(j.Path())); dir.Mode().Perm() != 0o700 {
		t.Fatalf("journal dir mode %v, want 0700", dir.Mode().Perm())
	}
	if err := j.ForgetAll(); reason(err) != task.ReasonBusy {
		t.Fatalf("ForgetAll while running = %v, want %s", err, task.ReasonBusy)
	}
	if err := j.Forget(a.ID); reason(err) != task.ReasonBusy {
		t.Fatalf("Forget while running = %v, want %s", err, task.ReasonBusy)
	}
	update(t, j, a.ID, task.Done, 1, loop.EndDone, t0)
	if _, err := j.Keep(a.ID, task.NewID(), t0); err != nil {
		t.Fatal(err)
	}
	if err := j.ForgetAll(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(j.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("delete all left the file: %v", err)
	}
	if ts, err := j.List(); len(ts) != 0 || err != nil {
		t.Fatalf("after delete all = %+v, %v", ts, err)
	}
	if ps, err := j.Projects(); len(ps) != 0 || err != nil {
		t.Fatalf("projects after delete all = %+v, %v", ps, err)
	}

	b := enqueue(t, j, window("org.example.Editor"), t0)
	update(t, j, b.ID, task.Stopped, 0, loop.EndStopped, t0)
	if err := j.Forget(b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(j.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("an empty journal left a file: %v", err)
	}
}

func TestProjectsKeepScopeAppAndStepsOnly(t *testing.T) {
	j := open(t)
	a := enqueue(t, j, window("org.example.Editor"), t0)
	if _, err := j.Keep(a.ID, task.NewID(), t0); reason(err) != protocol.CodeBadRequest {
		t.Fatalf("Keep of a running task = %v, want bad_request", err)
	}
	update(t, j, a.ID, task.Done, 7, loop.EndDone, t0.Add(time.Minute))
	p, err := j.Keep(a.ID, task.NewID(), t0.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if p.Scope != proto.ScopeWindow || p.App != "org.example.Editor" || p.Steps != 7 {
		t.Fatalf("project = %+v", p)
	}
	c, err := j.Continue(p.ID, task.NewID(), task.OriginApp, t0.Add(time.Hour))
	if err != nil || c.State != task.Working || c.Scope != p.Scope || c.App != p.App || c.Step != 0 {
		t.Fatalf("Continue = %+v, %v", c, err)
	}
	ps, _ := j.Projects()
	if len(ps) != 1 || !ps[0].Used.Equal(t0.Add(time.Hour)) {
		t.Fatalf("projects after Continue = %+v", ps)
	}
	if _, err := j.Continue(p.ID, task.NewID(), task.OriginApp, t0.Add(2*time.Hour)); err != nil {
		t.Fatalf("a continue while one runs should wait: %v", err)
	}
	if _, err := j.Continue(p.ID, task.NewID(), task.OriginApp, t0.Add(3*time.Hour)); reason(err) != task.ReasonFull {
		t.Fatalf("a continue past the queue = %v, want %s", err, task.ReasonFull)
	}
	if err := j.ForgetProject(p.ID); err != nil {
		t.Fatal(err)
	}
	if ps, _ := j.Projects(); len(ps) != 0 {
		t.Fatalf("projects after forget = %+v", ps)
	}
}

// The marker is a word the person typed, inside the shapes a goal, a title,
// a URL and a typed field take. No field may carry any of them to disk.
const marker = "zebrafinch"

var private = []string{
	"write to mum about the " + marker,       // a goal
	"Inbox — " + marker + " — Mail",          // a window title
	"https://mail.example.test/?q=" + marker, // a URL with its query
	"file:///home/me/" + marker + ".txt",     // a local URL
	marker + " " + marker,                    // typed words
	marker + "\t\n",                          // typed with control runes
}

func TestNoGoalTitleURLOrTypedWordReachesTheFile(t *testing.T) {
	j := open(t)
	// Run the whole scrub twice, across a restart, so a rewrite on the
	// second pass cannot bring anything back.
	for pass := range 2 {
		now := t0.Add(time.Duration(pass) * time.Hour)
		for _, v := range private {
			attempts := map[string]task.Task{
				"id":     {ID: v, Scope: proto.ScopeNested, Origin: task.OriginApp},
				"scope":  {ID: task.NewID(), Scope: v, Origin: task.OriginApp},
				"app":    {ID: task.NewID(), Scope: proto.ScopeWindow, App: v, Origin: task.OriginApp},
				"origin": {ID: task.NewID(), Scope: proto.ScopeNested, Origin: v},
				"reason": {ID: task.NewID(), Scope: proto.ScopeNested, Origin: task.OriginApp, Reason: v},
				"state":  {ID: task.NewID(), State: task.State(v), Scope: proto.ScopeNested, Origin: task.OriginApp},
			}
			for field, tk := range attempts {
				if _, err := j.Enqueue(tk, now); reason(err) != protocol.CodeBadRequest {
					t.Fatalf("pass %d: %s %q = %v, want bad_request", pass, field, v, err)
				}
			}
		}
		a := enqueue(t, j, window("org.example.Editor"), now)
		for _, v := range private {
			if _, err := j.Update(a.ID, task.NeedsYou, 1, v, now); reason(err) != protocol.CodeBadRequest {
				t.Fatalf("pass %d: reason %q = %v, want bad_request", pass, v, err)
			}
			if _, err := j.Update(a.ID, task.State(v), 1, "", now); reason(err) != protocol.CodeBadRequest {
				t.Fatalf("pass %d: state %q = %v, want bad_request", pass, v, err)
			}
		}
		update(t, j, a.ID, task.Done, 2, loop.EndDone, now)
		for _, v := range private {
			if _, err := j.Keep(a.ID, v, now); reason(err) != protocol.CodeBadRequest {
				t.Fatalf("pass %d: project id %q = %v, want bad_request", pass, v, err)
			}
		}
		p, err := j.Keep(a.ID, task.NewID(), now)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range private {
			if _, err := j.Continue(p.ID, task.NewID(), v, now); reason(err) != protocol.CodeBadRequest {
				t.Fatalf("pass %d: continue origin %q = %v, want bad_request", pass, v, err)
			}
			if _, err := j.Continue(p.ID, v, task.OriginApp, now); reason(err) != protocol.CodeBadRequest {
				t.Fatalf("pass %d: continue id %q = %v, want bad_request", pass, v, err)
			}
		}
		if _, err := j.Continue(p.ID, task.NewID(), task.OriginVoice, now); err != nil {
			t.Fatal(err)
		}
		j = task.Open(j.Path())
		if _, err := j.Recover(now.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		if err := j.Purge(now.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		scan(t, filepath.Dir(j.Path()))
	}
	ts, _ := j.List()
	ps, _ := j.Projects()
	if len(ts) != 4 || len(ps) != 2 {
		t.Fatalf("journal = %d tasks, %d projects; want 4 and 2", len(ts), len(ps))
	}
}

// scan fails if any file under dir holds the marker in any case, and lists
// the file's JSON keys against the fields a row may have.
func scan(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("journal dir holds %d files, want only the journal", len(entries))
	}
	data, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	low := strings.ToLower(string(data))
	if strings.Contains(low, marker) {
		t.Fatalf("the journal holds the typed marker:\n%s", data)
	}
	for _, key := range []string{`"goal"`, `"title"`, `"url"`, `"text"`, `"name"`, "http", "file:", "?q="} {
		if strings.Contains(low, key) {
			t.Fatalf("the journal holds %s:\n%s", key, data)
		}
	}
}
