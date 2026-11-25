package spec

import (
	"encoding/json"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestGuideNumbersAreTheResearchValues(t *testing.T) {
	g := mustLoad(t).Guide
	if g.MaxSteps != 12 || g.ReadTimeoutS != 75 || g.PickRepeatGuardMs != 1200 {
		t.Errorf("guide = %d steps, %gs read, %gms pick guard; want 12, 75, 1200", g.MaxSteps, g.ReadTimeoutS, g.PickRepeatGuardMs)
	}
	if want := (GuideKeep{ChipS: 12, MaxEntries: 80, MatchMin: 0.75, FindMin: 80, MissesToStale: 2}); g.Keep != want {
		t.Errorf("guide.keep = %+v, want %+v", g.Keep, want)
	}
	if want := (GuideSettle{WidthPx: 64, HeightPx: 40, MoveDelta: 4, StillDelta: 1.5, FrameMs: 150, NapMs: 450, MaxFrames: 20}); g.Settle != want {
		t.Errorf("guide.settle = %+v, want %+v", g.Settle, want)
	}
	if want := (GuideHints{CooldownS: 1200, RetractS: 45, OncePerKeyPerDay: true}); g.Hints != want {
		t.Errorf("guide.hints = %+v, want %+v", g.Hints, want)
	}
	if got := g.Settle.SettleBudgetMs(); got != 3000 {
		t.Errorf("settle budget %gms, want 3000", got)
	}
}

func TestGuidePhasesDeriveCharacterStates(t *testing.T) {
	s := mustLoad(t)
	want := map[string]string{
		"idle":       NoState,
		"reading":    "reading",
		"pointing":   "ready",
		"waiting":    "ready",
		"explaining": "ready",
		"done":       "ready",
		"nudging":    "ready",
		"keeping":    "thinking",
		"winding":    NoState,
	}
	if len(want) != len(GuidePhaseIDs) {
		t.Fatalf("table covers %d phases, GuidePhaseIDs lists %d", len(want), len(GuidePhaseIDs))
	}
	for _, phase := range GuidePhaseIDs {
		state, ok := s.GuideState(phase)
		if !ok || state != want[phase] {
			t.Errorf("GuideState(%q) = %q, %v; want %q, true", phase, state, ok, want[phase])
		}
		if state != NoState {
			if _, ok := s.State(state); !ok {
				t.Errorf("phase %s derives %q, which has no shape row", phase, state)
			}
		} else if s.Guide.Phases[phase].Reason == "" {
			t.Errorf("phase %s derives no state and gives no reason", phase)
		}
	}
	if _, ok := s.GuideState("wandering"); ok {
		t.Error("an unknown phase resolved")
	}
}

// guideLeaves lists every guide.* value path in the embedded spec.
func guideLeaves(t *testing.T) map[string]string {
	t.Helper()
	flat, err := flattenJSON(embedded)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for p, v := range flat {
		if strings.HasPrefix(p, "guide.") {
			out[p] = v
		}
	}
	if len(out) < 36 {
		t.Fatalf("only %d guide values found", len(out))
	}
	return out
}

func TestEveryGuideNumberRejectsANegative(t *testing.T) {
	leaves := guideLeaves(t)
	n := 0
	for _, p := range sortedKeys(leaves) {
		if _, err := strconv.ParseFloat(leaves[p], 64); err != nil {
			continue
		}
		n++
		t.Run(p, func(t *testing.T) {
			_, err := Parse(edited(t, func(r map[string]any) { setPath(r, p, json.Number("-1")) }))
			if err == nil || !strings.Contains(err.Error(), p+": negative value") {
				t.Fatalf("negative %s: got %v", p, err)
			}
		})
	}
	if n != 17 {
		t.Fatalf("checked %d guide numbers, want 17", n)
	}
}

func TestEveryGuideValueDriftBitesTheDoc(t *testing.T) {
	md, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatal(err)
	}
	leaves := guideLeaves(t)
	for _, p := range sortedKeys(leaves) {
		t.Run(p, func(t *testing.T) {
			var drifted any
			switch v := leaves[p]; v {
			case "true":
				drifted = false
			case "false":
				drifted = true
			default:
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					drifted = json.Number(strconv.FormatFloat(f+1, 'g', -1, 64))
				} else {
					drifted = v + "x"
				}
			}
			js := edited(t, func(r map[string]any) { setPath(r, p, drifted) })
			diffs, err := parity(md, js)
			if err != nil {
				t.Fatal(err)
			}
			if len(diffs) != 1 || !strings.HasPrefix(diffs[0], p+":") {
				t.Fatalf("want one drift on %s, got %q", p, diffs)
			}
		})
	}
	t.Run("doc row dropped", func(t *testing.T) {
		diffs, err := parity([]byte(replaceOnce(t, string(md), "| missesToStale | 2 |\n", "")), embedded)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(diffs, []string{"guide.keep.missesToStale: missing from the doc"}) {
			t.Fatalf("got %q", diffs)
		}
	})
}

func TestGuideLoadRejects(t *testing.T) {
	num := func(s string) json.Number { return json.Number(s) }
	set := func(path, v string) []byte {
		return edited(t, func(r map[string]any) { setPath(r, path, num(v)) })
	}
	phase := func(id, state, reason string) []byte {
		return edited(t, func(r map[string]any) {
			node(r, "guide.phases")[id] = map[string]any{"state": state, "reason": reason}
		})
	}
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"missing guide", edited(t, func(r map[string]any) { delete(r, "guide") }), "guide: missing key"},
		{"unknown guide key", edited(t, func(r map[string]any) { node(r, "guide")["hedgeS"] = num("9") }), "guide.hedgeS: unknown key"},
		{"missing keep store size", edited(t, func(r map[string]any) { delete(node(r, "guide.keep"), "maxEntries") }), "guide.keep.maxEntries: missing key"},
		{"zero steps", set("guide.maxSteps", "0"), "guide.maxSteps: must be at least 1"},
		{"fractional steps", set("guide.maxSteps", "12.5"), "guide.maxSteps: want an integer"},
		{"zero read timeout", set("guide.readTimeoutS", "0"), "guide.readTimeoutS: must be positive"},
		{"zero pick guard", set("guide.pickRepeatGuardMs", "0"), "guide.pickRepeatGuardMs: must be positive"},
		{"zero keep chip", set("guide.keep.chipS", "0"), "guide.keep.chipS: must be positive"},
		{"empty keep store", set("guide.keep.maxEntries", "0"), "guide.keep.maxEntries: must be at least 1"},
		{"fractional keep store", set("guide.keep.maxEntries", "80.5"), "guide.keep.maxEntries: want an integer"},
		{"zero match", set("guide.keep.matchMin", "0"), "guide.keep.matchMin: must lie in (0, 1]"},
		{"match past certainty", set("guide.keep.matchMin", "1.2"), "guide.keep.matchMin: must lie in (0, 1]"},
		{"zero find score", set("guide.keep.findMin", "0"), "guide.keep.findMin: must lie in (0, 100]"},
		{"find score past 100", set("guide.keep.findMin", "101"), "guide.keep.findMin: must lie in (0, 100]"},
		{"fractional find score", set("guide.keep.findMin", "80.5"), "guide.keep.findMin: want an integer"},
		{"never stale", set("guide.keep.missesToStale", "0"), "guide.keep.missesToStale: must be at least 1"},
		{"zero fingerprint width", set("guide.settle.widthPx", "0"), "guide.settle: a fingerprint needs widthPx and heightPx of at least 1"},
		{"zero fingerprint height", set("guide.settle.heightPx", "0"), "guide.settle: a fingerprint needs widthPx and heightPx of at least 1"},
		{"zero still delta", set("guide.settle.stillDelta", "0"), "guide.settle.stillDelta: must be positive"},
		{"move below still", set("guide.settle.moveDelta", "1"), "guide.settle.moveDelta: must lie in (stillDelta, 255]"},
		{"move equal to still", set("guide.settle.moveDelta", "1.5"), "guide.settle.moveDelta: must lie in (stillDelta, 255]"},
		{"move past the gray range", set("guide.settle.moveDelta", "256"), "guide.settle.moveDelta: must lie in (stillDelta, 255]"},
		{"zero frame", set("guide.settle.frameMs", "0"), "guide.settle.frameMs: must be positive"},
		{"nap shorter than a frame", set("guide.settle.napMs", "100"), "guide.settle.napMs: must last at least one frame"},
		{"no settle frames", set("guide.settle.maxFrames", "0"), "guide.settle.maxFrames: must be at least 1"},
		{"watcher shorter than the nap", set("guide.settle.maxFrames", "2"), "guide.settle.maxFrames: the watcher must outlast the nap"},
		{"zero hint cooldown", set("guide.hints.cooldownS", "0"), "guide.hints.cooldownS: must be positive"},
		{"zero hint retract", set("guide.hints.retractS", "0"), "guide.hints.retractS: must be positive"},
		{"hint outlives its cooldown", set("guide.hints.retractS", "1200"), "guide.hints.retractS: a hint must retract before the cooldown ends"},
		{"string once per day", edited(t, func(r map[string]any) { node(r, "guide.hints")["oncePerKeyPerDay"] = "yes" }), "guide.hints.oncePerKeyPerDay: want a boolean"},
		{"missing phase", edited(t, func(r map[string]any) { delete(node(r, "guide.phases"), "keeping") }), "guide.phases: missing [keeping]"},
		{"extra phase", phase("wandering", "ready", ""), "guide.phases: missing [], unknown [wandering]"},
		{"unknown derived state", phase("reading", "pondering", ""), `guide.phases.reading.state: unknown state "pondering"`},
		{"empty state", phase("reading", "", ""), `guide.phases.reading.state: unknown state ""`},
		{"no state and no reason", phase("winding", "none", ""), "guide.phases.winding: a phase that derives no state must give a reason"},
		{"idle with no reason", phase("idle", "none", ""), "guide.phases.idle: a phase that derives no state must give a reason"},
		{"reason beside a state", phase("pointing", "ready", "because"), "guide.phases.pointing.reason: only a phase that derives no state gives a reason"},
		{"idle with a state", phase("idle", "ready", ""), "guide.phases.idle: an idle guide derives no state"},
		{"missing reason key", edited(t, func(r map[string]any) { delete(node(r, "guide.phases.done"), "reason") }), "guide.phases.done.reason: missing key"},
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
