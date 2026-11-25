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

func hands(args []string, out, errs *bytes.Buffer) int { return runOniHands(args, out, errs) }

// handsRun runs one hands verb with --json, checks every key it printed is
// the wire's own, and decodes it strictly, checking it round-trips
func handsRun(t *testing.T, v any, args ...string) {
	t.Helper()
	var out, errs bytes.Buffer
	if code := hands(append(args, "--json"), &out, &errs); code != 0 {
		t.Fatalf("%v exited %d: %s", args, code, errs.String())
	}
	if errs.Len() != 0 {
		t.Fatalf("%v wrote to stderr in JSON mode: %q", args, errs.String())
	}
	handsKeys(t, out.Bytes())
	roundTrip(t, &out, v)
}

// handsKeys checks an answer holds only where Hands stands, a tab only its
// id, origin and title, and a ledger row only codes and numbers
func handsKeys(t *testing.T, printed []byte) {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(printed, &top); err != nil {
		t.Fatal(err)
	}
	lists := map[string][]string{
		"tabs":  {"id", "origin", "title"},
		"rows":  {"at", "task", "step", "tool", "outcome", "reason", "ms"},
		"tasks": {"id", "state", "scope", "app", "origin", "step", "at", "ended", "reason"},
	}
	for key, raw := range top {
		if !slices.Contains([]string{"state", "browser", "tabs", "origin", "step", "max", "pause", "line", "rows",
			"scope", "app", "task", "tasks", "missing"}, key) {
			t.Fatalf("the answer carries %q: %s", key, printed)
		}
		keys, ok := lists[key]
		if !ok {
			continue
		}
		var items []map[string]json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			t.Fatal(err)
		}
		for _, item := range items {
			for k := range item {
				if !slices.Contains(keys, k) {
					t.Fatalf("a listed %s carries %q: %s", key, k, printed)
				}
			}
		}
	}
}

// The CLI opens the Hands browser, lists its tabs, stops Hands and says
// where it stands, each answer round-tripping the wire; the log holds
// codes only.
func TestOniHandsCLIRoundTripsOverJSON(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOniHands)
	var status protocol.OniHandsResult
	handsRun(t, &status)
	if status.State != protocol.OniHandsStateClosed || status.Browser != "chromium" {
		t.Fatalf("status answered %+v", status)
	}
	var opened protocol.OniHandsResult
	handsRun(t, &opened, "open")
	if opened.State != protocol.OniHandsStateOpen || len(opened.Tabs) != 0 {
		t.Fatalf("open answered %+v", opened)
	}
	var listed protocol.OniHandsResult
	handsRun(t, &listed, "tabs")
	if len(listed.Tabs) != 2 || listed.Tabs[0].Origin != "http://127.0.0.1:8137" {
		t.Fatalf("tabs answered %+v", listed)
	}
	var stopped protocol.OniHandsResult
	handsRun(t, &stopped, "stop")
	if stopped.State != protocol.OniHandsStateOpen {
		t.Fatalf("stop answered %+v", stopped)
	}
	var again protocol.OniHandsResult
	handsRun(t, &again, "status")
	if again.State != protocol.OniHandsStateOpen {
		t.Fatalf("status after open answered %+v", again)
	}
	var log protocol.OniHandsLogResult
	handsRun(t, &log, "log")
	if len(log.Rows) == 0 {
		t.Fatal("the log is empty")
	}
	for _, row := range log.Rows {
		if !protocol.ValidOniHandsRow(row) {
			t.Fatalf("a row holds more than codes: %+v", row)
		}
	}

	var out, errs bytes.Buffer
	if code := hands([]string{"tabs"}, &out, &errs); code != 0 || errs.Len() != 0 {
		t.Fatalf("tabs exited %d: %s", code, errs.String())
	}
	if lines := strings.Split(strings.TrimSpace(out.String()), "\n"); len(lines) != 2 || lines[0] != "D1A2B3C4\thttp://127.0.0.1:8137\tContact us" {
		t.Fatalf("text tabs: %q", out.String())
	}
	out.Reset()
	if code := hands(nil, &out, &errs); code != 0 || out.String() != "The Hands browser is open, and no tab is authorized.\nBrowser: chromium\n" {
		t.Fatalf("text status exited %d: %q", code, out.String())
	}
	out.Reset()
	if code := hands([]string{"log"}, &out, &errs); code != 0 {
		t.Fatalf("text log exited %d: %s", code, errs.String())
	}
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if fields := strings.Split(line, "\t"); len(fields) != 7 || !strings.HasSuffix(fields[6], " ms") {
			t.Fatalf("a log line is not seven codes: %q", line)
		}
	}
}

// Authorize, start, resume and forget are the user's own press in the app
// or on Oni: the CLI refuses them before anything is sent, as it refuses
// an unknown verb, extra words and unknown flags.
func TestOniHandsCLIRefusesTheAppsPresses(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOniHands)
	for _, args := range [][]string{{"authorize"}, {"start"}, {"resume"}, {"forget"}, {"authorize", "--json"}} {
		var out, errs bytes.Buffer
		if code := hands(args, &out, &errs); code != 2 || out.Len() != 0 || !strings.Contains(errs.String(), "done in the Hyakki app") {
			t.Fatalf("%v exited %d: %q %q", args, code, out.String(), errs.String())
		}
	}
	for _, args := range [][]string{{"press"}, {"open", "now"}, {"--all"}, {"start", "fill the form"}} {
		var out, errs bytes.Buffer
		if code := hands(args, &out, &errs); code != 2 || out.Len() != 0 {
			t.Fatalf("%v exited %d: %q", args, code, out.String())
		}
	}
	var status protocol.OniHandsResult
	handsRun(t, &status)
	if status.State != protocol.OniHandsStateClosed {
		t.Fatalf("a refused verb reached the daemon: %+v", status)
	}
}

// With Hands off the daemon refuses every verb: the CLI exits 1 with its
// message and reason, and prints nothing on stdout.
func TestOniHandsCLIReportsARefusal(t *testing.T) {
	demoOniDaemon(t, demo.FeatureOni)
	for _, args := range [][]string{nil, {"open", "--json"}, {"log"}} {
		var out, errs bytes.Buffer
		if code := hands(args, &out, &errs); code != 1 || out.Len() != 0 || !strings.Contains(errs.String(), "(feature-off)") {
			t.Fatalf("%v exited %d: %q %q", args, code, out.String(), errs.String())
		}
	}
}
