package main

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"ryoku-rashin/internal/app/demo"
	"ryoku-rashin/internal/protocol"
)

// reflexRun runs one reflex verb with --json and decodes what it printed
// strictly, checking it round-trips
func reflexRun(t *testing.T, run func([]string, *bytes.Buffer, *bytes.Buffer) int, v any, args ...string) {
	t.Helper()
	var out, errs bytes.Buffer
	if code := run(append(args, "--json"), &out, &errs); code != 0 {
		t.Fatalf("%v exited %d: %s", args, code, errs.String())
	}
	if errs.Len() != 0 {
		t.Fatalf("%v wrote to stderr in JSON mode: %q", args, errs.String())
	}
	rawKeys(t, out.Bytes())
	roundTrip(t, &out, v)
}

func reminders(args []string, out, errs *bytes.Buffer) int { return runOniReminders(args, out, errs) }
func routines(args []string, out, errs *bytes.Buffer) int  { return runOniRoutines(args, out, errs) }
func reflexLog(args []string, out, errs *bytes.Buffer) int { return runOniReflex(args, out, errs) }

// rawKeys checks every object a list carries holds only the wire's own
// keys: a reminder its id, kind, due, own words and late mark; a routine
// its name, length and dates; a ledger row its command, route, timing,
// confidence and outcome. Nothing like a transcript rides any of them.
func rawKeys(t *testing.T, printed []byte) {
	t.Helper()
	allowed := map[string][]string{
		"reminders": {"id", "kind", "due", "text", "late"},
		"routines":  {"name", "steps", "saved", "used"},
		"rows":      {"at", "intent", "route", "ms", "confidence", "outcome"},
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(printed, &top); err != nil {
		t.Fatal(err)
	}
	for list, keys := range allowed {
		raw, ok := top[list]
		if !ok {
			continue
		}
		var items []map[string]json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			t.Fatal(err)
		}
		for _, item := range items {
			for key := range item {
				if !slices.Contains(keys, key) {
					t.Fatalf("a listed %s carries %q: %s", list, key, printed)
				}
			}
		}
	}
	for key := range top {
		if !slices.Contains([]string{"reminders", "noWake", "routines", "ran", "rows"}, key) {
			t.Fatalf("the answer carries %q: %s", key, printed)
		}
	}
}

// The reminders and timers list soonest first by their own words; one is
// cancelled by its id and the rest with --all, each answer the list left.
func TestOniRemindersListAndCancelOverJSON(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOniReflex)
	var listed protocol.OniRemindersResult
	reflexRun(t, reminders, &listed)
	if len(listed.Reminders) != 2 || listed.NoWake {
		t.Fatalf("listed %+v", listed)
	}
	timer, reminder := listed.Reminders[0], listed.Reminders[1]
	if timer.Kind != protocol.OniReminderKindTimer || timer.Text != "" || reminder.Kind != protocol.OniReminderKindReminder || reminder.Text != "call the dentist" || !timer.Due.Before(reminder.Due) {
		t.Fatalf("not soonest first by their own words: %+v", listed.Reminders)
	}

	var out, errs bytes.Buffer
	if code := runOniReminders(nil, &out, &errs); code != 0 || errs.Len() != 0 {
		t.Fatalf("list exited %d: %s", code, errs.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], timer.ID+"\ttimer\t") || !strings.HasPrefix(lines[1], reminder.ID+"\treminder\t") || !strings.HasSuffix(lines[1], "\tcall the dentist") {
		t.Fatalf("text list: %q", out.String())
	}

	var cancelled protocol.OniRemindersResult
	reflexRun(t, reminders, &cancelled, "--cancel", timer.ID)
	if len(cancelled.Reminders) != 1 || cancelled.Reminders[0].ID != reminder.ID {
		t.Fatalf("after cancelling the timer: %+v", cancelled)
	}
	out.Reset()
	errs.Reset()
	if code := runOniReminders([]string{"--cancel", timer.ID}, &out, &errs); code != 1 || out.Len() != 0 || !strings.Contains(errs.String(), "That reminder is not set.") {
		t.Fatalf("cancelling it again: exit %d, %q, %q", code, out.String(), errs.String())
	}
	out.Reset()
	if code := runOniReminders([]string{"--all"}, &out, &errs); code != 0 || out.String() != "Cancelled every reminder and timer.\n" {
		t.Fatalf("--all: exit %d, %q", code, out.String())
	}
	var empty protocol.OniRemindersResult
	reflexRun(t, reminders, &empty)
	if empty.Reminders == nil || len(empty.Reminders) != 0 {
		t.Fatalf("nothing left should print an empty list: %+v", empty)
	}
	out.Reset()
	if code := runOniReminders(nil, &out, &errs); code != 0 || out.String() != "No reminder or timer is set.\n" {
		t.Fatalf("empty text list: exit %d, %q", code, out.String())
	}
}

// Where the wake unit cannot be armed the answer says so, and the text
// list says Oni stays running while a reminder is set.
func TestOniRemindersSayWhenOniMustStayRunning(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOniReflexNoWake)
	var listed protocol.OniRemindersResult
	reflexRun(t, reminders, &listed)
	if !listed.NoWake || len(listed.Reminders) != 2 {
		t.Fatalf("listed %+v", listed)
	}
	var out, errs bytes.Buffer
	if code := runOniReminders(nil, &out, &errs); code != 0 || !strings.Contains(errs.String(), "Oni stays running while one is set") {
		t.Fatalf("text list: exit %d, stderr %q", code, errs.String())
	}
}

// Routines list by name and length, the most recently used first; --run
// replays one by name or the latest, --forget drops one, --all every one.
func TestOniRoutinesRunAndForgetOverJSON(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOniReflex)
	var listed protocol.OniRoutinesResult
	reflexRun(t, routines, &listed)
	if len(listed.Routines) != 2 || listed.Ran || listed.Routines[0].Name != "Morning" || listed.Routines[0].Steps != 3 {
		t.Fatalf("listed %+v", listed)
	}
	var ran protocol.OniRoutinesResult
	reflexRun(t, routines, &ran, "--run", "Wind down")
	if !ran.Ran || ran.Routines[0].Name != "Wind down" {
		t.Fatalf("run by name: %+v", ran)
	}
	var out, errs bytes.Buffer
	if code := runOniRoutines([]string{"--run"}, &out, &errs); code != 0 || out.String() != "Ran Wind down.\n" {
		t.Fatalf("run the latest: exit %d, %q, %q", code, out.String(), errs.String())
	}
	var forgot protocol.OniRoutinesResult
	reflexRun(t, routines, &forgot, "--forget", "Morning")
	if forgot.Ran || len(forgot.Routines) != 1 || forgot.Routines[0].Name != "Wind down" {
		t.Fatalf("after forgetting Morning: %+v", forgot)
	}
	out.Reset()
	errs.Reset()
	if code := runOniRoutines([]string{"--forget", "Morning"}, &out, &errs); code != 1 || out.Len() != 0 || !strings.Contains(errs.String(), "There is no routine by that name.") {
		t.Fatalf("forgetting it again: exit %d, %q, %q", code, out.String(), errs.String())
	}
	out.Reset()
	if code := runOniRoutines(nil, &out, &errs); code != 0 || !strings.HasPrefix(out.String(), "Wind down\t2 steps\tsaved ") {
		t.Fatalf("text list: exit %d, %q", code, out.String())
	}
	var none protocol.OniRoutinesResult
	reflexRun(t, routines, &none, "--all")
	if len(none.Routines) != 0 || none.Routines == nil {
		t.Fatalf("after --all: %+v", none)
	}
	errs.Reset()
	if code := runOniRoutines([]string{"--run"}, &out, &errs); code != 1 || errs.Len() == 0 {
		t.Fatalf("running with nothing kept: exit %d, %q", code, errs.String())
	}
}

// The ledger prints newest first, each row a command, its route, timing,
// confidence and outcome, and never words.
func TestOniReflexLogOverJSON(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOniReflex)
	var log protocol.OniReflexLogResult
	reflexRun(t, reflexLog, &log, "log")
	if len(log.Rows) != 4 || log.Rows[0].Outcome != protocol.OniReflexCooldown || !log.Rows[0].At.After(log.Rows[1].At) {
		t.Fatalf("log %+v", log)
	}
	var out, errs bytes.Buffer
	if code := runOniReflex([]string{"log"}, &out, &errs); code != 0 {
		t.Fatalf("text log exited %d: %s", code, errs.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 4 || !strings.Contains(lines[0], "\topen_app\trules\t2 ms\t97%\tcooldown") || !strings.Contains(lines[2], "\tvolume_up\tlocal\t140 ms\t82%\tdone") {
		t.Fatalf("text log: %q", out.String())
	}
}

// With quick commands off every verb is refused with the feature's reason
// and prints nothing on stdout.
func TestOniReflexVerbsRefuseWhileOff(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOni)
	for _, c := range []struct {
		run  func([]string, *bytes.Buffer, *bytes.Buffer) int
		args []string
		says string
	}{
		{reminders, nil, "Reminders are off"},
		{reminders, []string{"--all", "--json"}, "Reminders are off"},
		{routines, []string{"--run", "Morning"}, "Routines are off"},
		{reflexLog, []string{"log", "--json"}, "Quick commands are off"},
	} {
		var out, errs bytes.Buffer
		if code := c.run(c.args, &out, &errs); code != 1 || out.Len() != 0 || !strings.Contains(errs.String(), c.says) {
			t.Errorf("%v while off: exit %d, %q, %q", c.args, code, out.String(), errs.String())
		}
	}
}

// Bad flags and words are usage errors, exit 2, before anything is asked.
func TestOniReflexVerbsRefuseBadUsage(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOniReflex)
	const id = "a07e3c18d95b42f6"
	for _, c := range []struct {
		run  func([]string, *bytes.Buffer, *bytes.Buffer) int
		args []string
	}{
		{reminders, []string{"--cancel"}},
		{reminders, []string{"--cancel", "--json"}},
		{reminders, []string{"--cancel", "NOT-AN-ID"}},
		{reminders, []string{"--cancel", id, "--all"}},
		{reminders, []string{"list"}},
		{reminders, []string{"--forget", "Morning"}},
		{routines, []string{"--forget"}},
		{routines, []string{"--run", "Morning", "--all"}},
		{routines, []string{"--forget", "Morning", "--run"}},
		{routines, []string{"--forget", " Morning"}},
		{routines, []string{"--cancel", id}},
		{routines, []string{"Morning"}},
		{reflexLog, nil},
		{reflexLog, []string{"logs"}},
		{reflexLog, []string{"log", "again"}},
		{reflexLog, []string{"log", "--all"}},
	} {
		var out, errs bytes.Buffer
		if code := c.run(c.args, &out, &errs); code != 2 || out.Len() != 0 {
			t.Errorf("%v: exit %d, %q", c.args, code, out.String())
		}
	}
	// nothing was cancelled or forgotten by any of them
	var listed protocol.OniRemindersResult
	reflexRun(t, reminders, &listed)
	var kept protocol.OniRoutinesResult
	reflexRun(t, routines, &kept)
	if len(listed.Reminders) != 2 || len(kept.Routines) != 2 {
		t.Fatalf("a usage error changed something: %+v, %+v", listed, kept)
	}
}
