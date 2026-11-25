package spec

import (
	"encoding/json"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

var phase16Sections = []string{"memory", "recall", "rewrite", "focus", "assignment"}

// phase16Leaves lists every Phase 16 value path in the embedded spec.
func phase16Leaves(t *testing.T) map[string]string {
	t.Helper()
	flat, err := flattenJSON(embedded)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for p, v := range flat {
		for _, s := range phase16Sections {
			if strings.HasPrefix(p, s+".") {
				out[p] = v
			}
		}
	}
	if len(out) != 25 {
		t.Fatalf("found %d Phase 16 values, want 25", len(out))
	}
	return out
}

// The draft numbers, as the research gives them and the owner has yet to
// ratify; every consumer reads them through these fields.
func TestPhase16NumbersAreTheDraftValues(t *testing.T) {
	s := mustLoad(t)
	if s.Memory != (Memory{CorrectS: 120, ForgetMax: 5}) {
		t.Errorf("memory = %+v", s.Memory)
	}
	if s.Recall != (Recall{Passages: 8, PassageTokens: 350, PromptBytes: 28672, AnswerTokens: 500, RequestS: 30}) {
		t.Errorf("recall = %+v", s.Recall)
	}
	if s.Rewrite != (Rewrite{MaxRunes: 8000, RequestS: 60, UndoS: 60}) {
		t.Errorf("rewrite = %+v", s.Rewrite)
	}
	if s.Focus != (Focus{MaxMinutes: 480, ReopenS: 600}) {
		t.Errorf("focus = %+v", s.Focus)
	}
	want := Assignment{
		Gate:       AssignmentGate{WidthPx: 48, HeightPx: 27, StillDelta: 3, GoneFraction: 0.5, CriticalTokens: 1},
		CacheViews: 30, AttemptsPerDay: 60, TokensPerDay: 200000, HedgeS: 4, FastS: 30, CheckerS: 60,
		Steps: AssignmentSteps{Min: 3, Max: 8},
	}
	if s.Assignment != want {
		t.Errorf("assignment = %+v", s.Assignment)
	}
	for _, d := range []struct {
		name      string
		got, want time.Duration
	}{
		{"correct", s.Memory.Correct(), 2 * time.Minute},
		{"recall request", s.Recall.Request(), 30 * time.Second},
		{"rewrite request", s.Rewrite.Request(), time.Minute},
		{"undo", s.Rewrite.Undo(), time.Minute},
		{"guard", s.Focus.Max(), 8 * time.Hour},
		{"reopen", s.Focus.Reopen(), 10 * time.Minute},
		{"hedge", s.Assignment.Hedge(), 4 * time.Second},
		{"fast lane", s.Assignment.Fast(), 30 * time.Second},
		{"checker lane", s.Assignment.Checker(), time.Minute},
	} {
		if d.got != d.want {
			t.Errorf("%s = %v, want %v", d.name, d.got, d.want)
		}
	}
}

// The focus section names no colour: the guard's look comes from the
// renderer's palette roles.
func TestFocusSectionHoldsNoColour(t *testing.T) {
	for p, v := range phase16Leaves(t) {
		if strings.HasPrefix(v, "#") || strings.Contains(strings.ToLower(p), "colo") {
			t.Errorf("%s = %s sets a colour", p, v)
		}
	}
}

func TestEveryPhase16NumberRejectsANegative(t *testing.T) {
	leaves := phase16Leaves(t)
	for _, p := range sortedKeys(leaves) {
		t.Run(p, func(t *testing.T) {
			_, err := Parse(edited(t, func(r map[string]any) { setPath(r, p, json.Number("-1")) }))
			if err == nil || !strings.Contains(err.Error(), p+": negative value") {
				t.Fatalf("negative %s: got %v", p, err)
			}
		})
	}
}

func TestEveryPhase16ValueDriftBitesTheDoc(t *testing.T) {
	md, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatal(err)
	}
	if diffs, err := parity(md, embedded); err != nil || len(diffs) > 0 {
		t.Fatalf("doc and json disagree: %q %v", diffs, err)
	}
	leaves := phase16Leaves(t)
	for _, p := range sortedKeys(leaves) {
		t.Run(p, func(t *testing.T) {
			f, err := strconv.ParseFloat(leaves[p], 64)
			if err != nil {
				t.Fatalf("%s = %q is not a number", p, leaves[p])
			}
			drifted := json.Number(strconv.FormatFloat(f+1, 'g', -1, 64))
			diffs, err := parity(md, edited(t, func(r map[string]any) { setPath(r, p, drifted) }))
			if err != nil {
				t.Fatal(err)
			}
			if len(diffs) != 1 || !strings.HasPrefix(diffs[0], p+":") {
				t.Fatalf("want one drift on %s, got %q", p, diffs)
			}
		})
	}
	for _, row := range []struct{ line, path string }{
		{"| correctS | 120 |\n", "memory.correctS"},
		{"| promptBytes | 28672 |\n", "recall.promptBytes"},
		{"| maxRunes | 8000 |\n", "rewrite.maxRunes"},
		{"| maxMinutes | 480 |\n", "focus.maxMinutes"},
		{"| attemptsPerDay | 60 |\n", "assignment.attemptsPerDay"},
		{"| stillDelta | 3 |\n", "assignment.gate.stillDelta"},
	} {
		t.Run("doc row dropped "+row.path, func(t *testing.T) {
			diffs, err := parity([]byte(replaceOnce(t, string(md), row.line, "")), embedded)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(diffs, []string{row.path + ": missing from the doc"}) {
				t.Fatalf("got %q", diffs)
			}
		})
	}
}

func TestPhase16LoadAcceptsTheEdges(t *testing.T) {
	set := func(path, v string) []byte {
		return edited(t, func(r map[string]any) { setPath(r, path, json.Number(v)) })
	}
	for _, c := range []struct {
		name string
		data []byte
	}{
		{"a correction window at its ceiling", set("memory.correctS", "600")},
		{"a rewrite at its ceiling", set("rewrite.maxRunes", "20000")},
		{"a guard at its ceiling", set("focus.maxMinutes", "720")},
		{"reopen as long as the guard", set("focus.reopenS", "28800")},
		{"a thumbnail at its ceiling", set("assignment.gate.widthPx", "128")},
		{"a still delta at its ceiling", set("assignment.gate.stillDelta", "255")},
		{"every word gone", set("assignment.gate.goneFraction", "1")},
		{"no critical token may change", set("assignment.gate.criticalTokens", "0")},
		{"hedge just inside the fast lane", set("assignment.hedgeS", "29")},
		{"one step count", set("assignment.steps.min", "8")},
	} {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Parse(c.data); err != nil {
				t.Fatalf("refused: %v", err)
			}
		})
	}
}

func TestPhase16LoadRejects(t *testing.T) {
	set := func(path, v string) []byte {
		return edited(t, func(r map[string]any) { setPath(r, path, json.Number(v)) })
	}
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"missing memory", edited(t, func(r map[string]any) { delete(r, "memory") }), "memory: missing key"},
		{"missing focus", edited(t, func(r map[string]any) { delete(r, "focus") }), "focus: missing key"},
		{"a colour in focus", edited(t, func(r map[string]any) { node(r, "focus")["armed"] = "#F2735E" }), "focus.armed: unknown key"},
		{"unknown rewrite key", edited(t, func(r map[string]any) { node(r, "rewrite")["clipboard"] = true }), "rewrite.clipboard: unknown key"},
		{"missing gate key", edited(t, func(r map[string]any) { delete(node(r, "assignment.gate"), "goneFraction") }), "assignment.gate.goneFraction: missing key"},
		{"no correction window", set("memory.correctS", "0"), "memory.correctS: must be positive"},
		{"a long correction window", set("memory.correctS", "601"), "memory.correctS: at most 600"},
		{"fractional correction window", set("memory.correctS", "120.5"), "memory.correctS: want an integer"},
		{"no forget", set("memory.forgetMax", "0"), "memory.forgetMax: must be positive"},
		{"no passages", set("recall.passages", "0"), "recall.passages: must be positive"},
		{"no prompt", set("recall.promptBytes", "0"), "recall.promptBytes: must be positive"},
		{"no rewrite", set("rewrite.maxRunes", "0"), "rewrite.maxRunes: must be positive"},
		{"a huge rewrite", set("rewrite.maxRunes", "20001"), "rewrite.maxRunes: at most 20000"},
		{"no undo", set("rewrite.undoS", "0"), "rewrite.undoS: must be positive"},
		{"no guard", set("focus.maxMinutes", "0"), "focus.maxMinutes: must be positive"},
		{"a guard past a day's work", set("focus.maxMinutes", "721"), "focus.maxMinutes: at most 720"},
		{"reopen past the guard", set("focus.reopenS", "28801"), "focus.reopenS: no longer than the guard itself"},
		{"a big thumbnail", set("assignment.gate.heightPx", "129"), "assignment.gate: at most 128 px a side"},
		{"no thumbnail", set("assignment.gate.widthPx", "0"), "assignment.gate.widthPx: must be positive"},
		{"no still delta", set("assignment.gate.stillDelta", "0"), "assignment.gate.stillDelta: want 0 < stillDelta <= 255"},
		{"a still delta past grey", set("assignment.gate.stillDelta", "256"), "assignment.gate.stillDelta: want 0 < stillDelta <= 255"},
		{"no words gone", set("assignment.gate.goneFraction", "0"), "assignment.gate.goneFraction: want 0 < goneFraction <= 1"},
		{"more than every word gone", set("assignment.gate.goneFraction", "1.5"), "assignment.gate.goneFraction: want 0 < goneFraction <= 1"},
		{"fractional critical tokens", set("assignment.gate.criticalTokens", "1.5"), "assignment.gate.criticalTokens: want an integer"},
		{"no cache", set("assignment.cacheViews", "0"), "assignment.cacheViews: must be positive"},
		{"no attempts", set("assignment.attemptsPerDay", "0"), "assignment.attemptsPerDay: must be positive"},
		{"no tokens", set("assignment.tokensPerDay", "0"), "assignment.tokensPerDay: must be positive"},
		{"hedge after the fast lane", set("assignment.hedgeS", "30"), "assignment.hedgeS: must come before the fast lane's time is up"},
		{"no checker", set("assignment.checkerS", "0"), "assignment.checkerS: must be positive"},
		{"steps upside down", set("assignment.steps.min", "9"), "assignment.steps: min above max"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, err := Parse(c.data)
			if err == nil {
				t.Fatalf("accepted; got spec version %d", s.Version)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %q does not name %q", err, c.want)
			}
		})
	}
}
