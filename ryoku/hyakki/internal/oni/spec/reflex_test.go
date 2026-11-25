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

func TestReflexNumbersAreTheResearchValues(t *testing.T) {
	r := mustLoad(t).Reflex
	want := Reflex{
		MaxActions:          6,
		MaxActionsPerMinute: 12,
		LaunchCooldownS:     4,
		DanglingVerbS:       6,
		EarlyFireMin:        0.8,
		ExactAppMin:         0.95,
		FuzzyMin:            0.6,
		FuzzyMargin:         0.1,
		SiteStickyS:         120,
		CatalogMax:          200,
		Routines:            ReflexRoutines{OfferMinSteps: 3, GapS: 20, MaxRoutines: 20, MaxSteps: 8, ChipS: 12},
		Reminders:           ReflexReminders{MaxDays: 7, MaxTimerH: 24, BusyTries: 4, BusyWaitS: 15, WakeHorizonMin: 30, ResidencyHorizonMin: 35, MaxPending: 50},
		Decisions:           ReflexDecisions{MinConfidence: 0.7, MinMargin: 0.15, BudgetMs: 1500, HostedPerMin: 6},
		Ledger:              ReflexLedger{MaxRows: 500},
	}
	if r != want {
		t.Fatalf("reflex = %+v\nwant %+v", r, want)
	}
}

func TestReflexDurationsReadTheSpec(t *testing.T) {
	r := mustLoad(t).Reflex
	for _, c := range []struct {
		name      string
		got, want time.Duration
	}{
		{"launch cooldown", r.LaunchCooldown(), 4 * time.Second},
		{"dangling verb", r.DanglingVerb(), 6 * time.Second},
		{"site sticky", r.SiteSticky(), 2 * time.Minute},
		{"routine gap", r.Routines.Gap(), 20 * time.Second},
		{"routine chip", r.Routines.Chip(), 12 * time.Second},
		{"longest reminder", r.Reminders.MaxReminder(), 7 * 24 * time.Hour},
		{"longest timer", r.Reminders.MaxTimer(), 24 * time.Hour},
		{"busy wait", r.Reminders.BusyWait(), 15 * time.Second},
		{"wake horizon", r.Reminders.WakeHorizon(), 30 * time.Minute},
		{"residency horizon", r.Reminders.ResidencyHorizon(), 35 * time.Minute},
		{"decision budget", r.Decisions.Budget(), 1500 * time.Millisecond},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
	if !(r.Reminders.MaxTimer() < r.Reminders.MaxReminder()) {
		t.Error("the longest timer is not shorter than the longest reminder")
	}
	if !(r.Reminders.ResidencyHorizon() > r.Reminders.WakeHorizon()) {
		t.Error("the residency horizon does not outlast the wake horizon")
	}
}

// reflexLeaves lists every reflex.* value path in the embedded spec.
func reflexLeaves(t *testing.T) map[string]string {
	t.Helper()
	flat, err := flattenJSON(embedded)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for p, v := range flat {
		if strings.HasPrefix(p, "reflex.") {
			out[p] = v
		}
	}
	if len(out) != 27 {
		t.Fatalf("found %d reflex values, want 27", len(out))
	}
	return out
}

func TestEveryReflexNumberRejectsANegative(t *testing.T) {
	leaves := reflexLeaves(t)
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
	if n != 27 {
		t.Fatalf("checked %d reflex numbers, want 27", n)
	}
}

func TestEveryReflexValueDriftBitesTheDoc(t *testing.T) {
	md, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatal(err)
	}
	if diffs, err := parity(md, embedded); err != nil || len(diffs) > 0 {
		t.Fatalf("doc and json disagree: %q %v", diffs, err)
	}
	leaves := reflexLeaves(t)
	for _, p := range sortedKeys(leaves) {
		t.Run(p, func(t *testing.T) {
			f, err := strconv.ParseFloat(leaves[p], 64)
			if err != nil {
				t.Fatalf("%s = %q is not a number", p, leaves[p])
			}
			drifted := json.Number(strconv.FormatFloat(f+1, 'g', -1, 64))
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
		diffs, err := parity([]byte(replaceOnce(t, string(md), "| residencyHorizonMin | 35 |\n", "")), embedded)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(diffs, []string{"reflex.reminders.residencyHorizonMin: missing from the doc"}) {
			t.Fatalf("got %q", diffs)
		}
	})
}

func TestReflexLoadRejects(t *testing.T) {
	num := func(s string) json.Number { return json.Number(s) }
	set := func(path, v string) []byte {
		return edited(t, func(r map[string]any) { setPath(r, path, num(v)) })
	}
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"missing reflex", edited(t, func(r map[string]any) { delete(r, "reflex") }), "reflex: missing key"},
		{"unknown reflex key", edited(t, func(r map[string]any) { node(r, "reflex")["typeText"] = true }), "reflex.typeText: unknown key"},
		{"missing reminder store size", edited(t, func(r map[string]any) { delete(node(r, "reflex.reminders"), "maxPending") }), "reflex.reminders.maxPending: missing key"},

		{"no actions", set("reflex.maxActions", "0"), "reflex.maxActions: must be at least 1"},
		{"fractional actions", set("reflex.maxActions", "6.5"), "reflex.maxActions: want an integer"},
		{"minute cap below one utterance", set("reflex.maxActionsPerMinute", "5"), "reflex.maxActionsPerMinute: must admit one full utterance"},
		{"no minute cap", set("reflex.maxActionsPerMinute", "0"), "reflex.maxActionsPerMinute: must admit one full utterance"},
		{"fractional minute cap", set("reflex.maxActionsPerMinute", "12.5"), "reflex.maxActionsPerMinute: want an integer"},
		{"zero launch cooldown", set("reflex.launchCooldownS", "0"), "reflex.launchCooldownS: must be positive"},
		{"zero dangling verb", set("reflex.danglingVerbS", "0"), "reflex.danglingVerbS: must be positive"},
		{"zero early fire", set("reflex.earlyFireMin", "0"), "reflex.earlyFireMin: must lie in (0, 1]"},
		{"early fire past certainty", set("reflex.earlyFireMin", "1.2"), "reflex.earlyFireMin: must lie in (0, 1]"},
		{"exact app below early fire", set("reflex.exactAppMin", "0.7"), "reflex.exactAppMin: must lie in [earlyFireMin, 1]"},
		{"exact app past certainty", set("reflex.exactAppMin", "1.1"), "reflex.exactAppMin: must lie in [earlyFireMin, 1]"},
		{"zero fuzzy", set("reflex.fuzzyMin", "0"), "reflex.fuzzyMin: must lie in (0, exactAppMin)"},
		{"fuzzy as good as exact", set("reflex.fuzzyMin", "0.95"), "reflex.fuzzyMin: must lie in (0, exactAppMin)"},
		{"zero fuzzy margin", set("reflex.fuzzyMargin", "0"), "reflex.fuzzyMargin: must be positive with fuzzyMin + fuzzyMargin <= 1"},
		{"fuzzy margin past certainty", set("reflex.fuzzyMargin", "0.5"), "reflex.fuzzyMargin: must be positive with fuzzyMin + fuzzyMargin <= 1"},
		{"zero site sticky", set("reflex.siteStickyS", "0"), "reflex.siteStickyS: must be positive"},
		{"empty catalog", set("reflex.catalogMax", "0"), "reflex.catalogMax: must be at least 1"},
		{"fractional catalog", set("reflex.catalogMax", "200.5"), "reflex.catalogMax: want an integer"},

		{"one-step routine offer", set("reflex.routines.offerMinSteps", "1"), "reflex.routines.offerMinSteps: must lie in [2, maxSteps]"},
		{"offer longer than a routine", set("reflex.routines.offerMinSteps", "9"), "reflex.routines.offerMinSteps: must lie in [2, maxSteps]"},
		{"zero routine gap", set("reflex.routines.gapS", "0"), "reflex.routines.gapS: must be positive"},
		{"no routines", set("reflex.routines.maxRoutines", "0"), "reflex.routines.maxRoutines: must be at least 1"},
		{"no routine steps", set("reflex.routines.maxSteps", "0"), "reflex.routines.maxSteps: must be at least 1"},
		{"routine shorter than its offer", set("reflex.routines.maxSteps", "2"), "reflex.routines.offerMinSteps: must lie in [2, maxSteps]"},
		{"zero routine chip", set("reflex.routines.chipS", "0"), "reflex.routines.chipS: must be positive"},

		{"no reminder days", set("reflex.reminders.maxDays", "0"), "reflex.reminders.maxDays: must be at least 1"},
		{"reminder no longer than a timer", set("reflex.reminders.maxDays", "1"), "reflex.reminders.maxTimerH: a timer must be shorter than the longest reminder"},
		{"zero timer", set("reflex.reminders.maxTimerH", "0"), "reflex.reminders.maxTimerH: must be positive"},
		{"timer as long as a reminder", set("reflex.reminders.maxTimerH", "168"), "reflex.reminders.maxTimerH: a timer must be shorter than the longest reminder"},
		{"no busy tries", set("reflex.reminders.busyTries", "0"), "reflex.reminders.busyTries: must be at least 1"},
		{"busy tries past the wake horizon", set("reflex.reminders.busyTries", "120"), "reflex.reminders.busyWaitS: the busy wait (busyTries·busyWaitS) must end within the wake horizon"},
		{"zero busy wait", set("reflex.reminders.busyWaitS", "0"), "reflex.reminders.busyWaitS: must be positive"},
		{"busy wait past the wake horizon", set("reflex.reminders.busyWaitS", "450"), "reflex.reminders.busyWaitS: the busy wait (busyTries·busyWaitS) must end within the wake horizon"},
		{"zero wake horizon", set("reflex.reminders.wakeHorizonMin", "0"), "reflex.reminders.wakeHorizonMin: must be positive"},
		{"wake horizon reaches residency", set("reflex.reminders.wakeHorizonMin", "35"), "reflex.reminders.residencyHorizonMin: the daemon must stay resident past the wake horizon"},
		{"residency inside the wake horizon", set("reflex.reminders.residencyHorizonMin", "30"), "reflex.reminders.residencyHorizonMin: the daemon must stay resident past the wake horizon"},
		{"zero residency", set("reflex.reminders.residencyHorizonMin", "0"), "reflex.reminders.residencyHorizonMin: the daemon must stay resident past the wake horizon"},
		{"no pending reminders", set("reflex.reminders.maxPending", "0"), "reflex.reminders.maxPending: must be at least 1"},
		{"fractional pending", set("reflex.reminders.maxPending", "50.5"), "reflex.reminders.maxPending: want an integer"},

		{"zero decision confidence", set("reflex.decisions.minConfidence", "0"), "reflex.decisions.minConfidence: must lie in (0, 1]"},
		{"decision confidence past certainty", set("reflex.decisions.minConfidence", "1.5"), "reflex.decisions.minConfidence: must lie in (0, 1]"},
		{"zero decision margin", set("reflex.decisions.minMargin", "0"), "reflex.decisions.minMargin: must lie in (0, 1)"},
		{"whole decision margin", set("reflex.decisions.minMargin", "1"), "reflex.decisions.minMargin: must lie in (0, 1)"},
		{"zero decision budget", set("reflex.decisions.budgetMs", "0"), "reflex.decisions.budgetMs: must lie in [100, 5000]"},
		{"decision budget too short", set("reflex.decisions.budgetMs", "99"), "reflex.decisions.budgetMs: must lie in [100, 5000]"},
		{"decision budget too long", set("reflex.decisions.budgetMs", "5001"), "reflex.decisions.budgetMs: must lie in [100, 5000]"},
		{"fractional decision budget", set("reflex.decisions.budgetMs", "1500.5"), "reflex.decisions.budgetMs: want an integer"},
		{"no hosted decisions", set("reflex.decisions.hostedPerMin", "0"), "reflex.decisions.hostedPerMin: must be at least 1"},
		{"hosted decisions past the action cap", set("reflex.decisions.hostedPerMin", "13"), "reflex.decisions.hostedPerMin: must not exceed maxActionsPerMinute"},

		{"empty ledger", set("reflex.ledger.maxRows", "0"), "reflex.ledger.maxRows: must be at least 1"},
		{"fractional ledger", set("reflex.ledger.maxRows", "500.5"), "reflex.ledger.maxRows: want an integer"},
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
