package main

import (
	"bytes"
	"strings"
	"testing"

	"ryoku-rashin/internal/app/demo"
	"ryoku-rashin/internal/protocol"
)

// At the app stage the CLI says where Hands stands by the window's app id,
// lists the task journal with codes only, stops a paused task, and every
// answer round-trips the wire without a title, goal or window list.
func TestOniHandsCLIAtTheAppStage(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOniHandsAppPaused)
	var status protocol.OniHandsResult
	handsRun(t, &status)
	if status.State != protocol.OniHandsStatePaused || status.Scope != protocol.OniHandsScopeWindow || status.App != "org.gnome.TextEditor" ||
		status.Pause != protocol.OniHandsPauseKey || status.Task == "" || status.Browser != "" {
		t.Fatalf("status answered %+v", status)
	}
	var out, errs bytes.Buffer
	if code := hands(nil, &out, &errs); code != 0 || out.String() != "Paused in the window of org.gnome.TextEditor at step 4 of 25 (key).\n" {
		t.Fatalf("text status exited %d: %q %q", code, out.String(), errs.String())
	}

	var tasks protocol.OniHandsResult
	handsRun(t, &tasks, "tasks")
	if len(tasks.Tasks) != 4 || tasks.Tasks[0].ID != status.Task || tasks.Tasks[0].State != "needsyou" {
		t.Fatalf("tasks answered %+v", tasks.Tasks)
	}
	out.Reset()
	if code := hands([]string{"tasks"}, &out, &errs); code != 0 {
		t.Fatalf("text tasks exited %d: %s", code, errs.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("text tasks: %q", out.String())
	}
	for _, line := range lines {
		if fields := strings.Split(line, "\t"); len(fields) != 9 || len(fields[0]) != 32 {
			t.Fatalf("a task line is not nine codes: %q", line)
		}
		for _, word := range []string{"Notes", "Example Domain"} {
			if strings.Contains(line, word) {
				t.Fatalf("a task line carries a window's title: %q", line)
			}
		}
	}

	var stopped protocol.OniHandsResult
	handsRun(t, &stopped, "stop")
	if stopped.State != protocol.OniHandsStateAuthorized || stopped.Task != "" || stopped.App != "org.gnome.TextEditor" {
		t.Fatalf("stop answered %+v", stopped)
	}
	var after protocol.OniHandsResult
	handsRun(t, &after, "tasks")
	if after.Tasks[0].State != "stopped" || after.Tasks[0].Reason != protocol.OniHandsEndStopped || after.Tasks[0].Ended.IsZero() {
		t.Fatalf("the stopped task reads %+v", after.Tasks[0])
	}
	out.Reset()
	if code := hands(nil, &out, &errs); code != 0 || out.String() != "Authorized window: org.gnome.TextEditor. No task is running.\n" {
		t.Fatalf("text status after stop exited %d: %q", code, out.String())
	}
}

// Listing and picking windows and deleting the journal are the app's: the
// CLI refuses them before anything is sent.
func TestOniHandsCLIRefusesTheAppsWindowAndJournalPresses(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOniHandsApp)
	for _, args := range [][]string{{"windows"}, {"select"}, {"clear"}, {"windows", "--json"}} {
		var out, errs bytes.Buffer
		if code := hands(args, &out, &errs); code != 2 || out.Len() != 0 || !strings.Contains(errs.String(), "done in the Hyakki app") {
			t.Fatalf("%v exited %d: %q %q", args, code, out.String(), errs.String())
		}
	}
	var status protocol.OniHandsResult
	handsRun(t, &status)
	if status.State != protocol.OniHandsStateOpen || status.App != "" {
		t.Fatalf("a refused verb reached the daemon: %+v", status)
	}
	var out, errs bytes.Buffer
	if code := hands(nil, &out, &errs); code != 0 || out.String() != "No window is authorized.\n" {
		t.Fatalf("text status exited %d: %q", code, out.String())
	}
}

// At the desktop stage the CLI says whether the separate desktop can run
// and names what it lacks, never a browser.
func TestOniHandsCLIAtTheDesktopStage(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOniHandsDesktop)
	var status protocol.OniHandsResult
	handsRun(t, &status)
	if status.State != protocol.OniHandsStateAuthorized || len(status.Missing) != 0 || status.Browser != "" {
		t.Fatalf("status answered %+v", status)
	}
	var out, errs bytes.Buffer
	if code := hands(nil, &out, &errs); code != 0 || out.String() != "The separate desktop is ready. No task is running.\n" {
		t.Fatalf("text status exited %d: %q", code, out.String())
	}

	demoOniDaemon(t, demo.FeatureOniHandsNoNest)
	var lacking protocol.OniHandsResult
	handsRun(t, &lacking)
	if lacking.State != protocol.OniHandsStateClosed || strings.Join(lacking.Missing, ",") != "weston,niri" {
		t.Fatalf("status without the desktop answered %+v", lacking)
	}
	out.Reset()
	want := "The separate desktop cannot run.\nThe separate desktop needs weston, niri, which this computer lacks. Hyakki does not install them.\n"
	if code := hands(nil, &out, &errs); code != 0 || out.String() != want {
		t.Fatalf("text status exited %d: %q", code, out.String())
	}
}
