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

func TestHandsNumbersAreTheDraftValues(t *testing.T) {
	h := mustLoad(t).Hands
	want := Hands{
		Budget:         HandsBudget{Steps: 15, Extra: 10},
		RepeatLimit:    3,
		DoneRejections: 2,
		ResumeS:        300,
		FollowUpS:      120,
		HistoryCarry:   6,
		ConsentTTLS:    120,
		ReadTimeoutS:   75,
		ReadbackMs:     1500,
		ActMs:          3000,
		StopBudgetMs:   250,
		Page:           HandsPage{MaxControls: 200, MaxNameRunes: 80, MaxTextRunes: 6000, MaxFillRunes: 2000},
		Glow:           HandsGlow{FPS: 12, Low: 0.72, High: 1, PeriodS: 3.2, StripPx: 130, WaitingFactor: 0.5},
		Ledger:         HandsLedger{MaxRows: 500},
		App: HandsApp{GeometryTolPx: 2, VerifyMs: 500, SettleMs: 50, ChunkMs: 5000, ReadbackMs: 4000,
			TypeChunkRunes: 40, MaxTypeRunes: 200},
		Desktop:   HandsDesktop{WidthPx: 1280, HeightPx: 800, ReadyS: 20, StopGraceS: 3, IdleS: 600},
		Task:      HandsTask{Turns: 40, Ms: 600000, HostedUsd: 2, QueueMax: 1},
		Retention: HandsRetention{Days: 30, Rows: 200},
	}
	if h != want {
		t.Fatalf("hands = %+v\nwant %+v", h, want)
	}
}

func TestHandsDurationsReadTheSpec(t *testing.T) {
	h := mustLoad(t).Hands
	for _, c := range []struct {
		name      string
		got, want time.Duration
	}{
		{"resume window", h.Resume(), 5 * time.Minute},
		{"follow-up window", h.FollowUp(), 2 * time.Minute},
		{"consent card", h.ConsentTTL(), 2 * time.Minute},
		{"model request", h.ReadTimeout(), 75 * time.Second},
		{"read-back", h.Readback(), 1500 * time.Millisecond},
		{"one action", h.Act(), 3 * time.Second},
		{"stop", h.StopBudget(), 250 * time.Millisecond},
		{"glow breath", h.Glow.Period(), 3200 * time.Millisecond},
		{"glow frame", h.Glow.Frame(), time.Second / 12},
		{"window check", h.App.Verify(), 500 * time.Millisecond},
		{"keymap wait", h.App.Settle(), 50 * time.Millisecond},
		{"typing call", h.App.Chunk(), 5 * time.Second},
		{"typed read-back", h.App.Readback(), 4 * time.Second},
		{"nested ready", h.Desktop.Ready(), 20 * time.Second},
		{"nested stop grace", h.Desktop.StopGrace(), 3 * time.Second},
		{"nested idle", h.Desktop.Idle(), 10 * time.Minute},
		{"task time", h.Task.Time(), 10 * time.Minute},
		{"retention", h.Retention.Age(), 30 * 24 * time.Hour},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
	if got := h.Budget.Max(); got != 25 {
		t.Errorf("budget max = %d, want 25", got)
	}
	if !(h.StopBudget() < h.Readback() && h.Readback() < h.Act()) {
		t.Error("a stop does not outrun every step")
	}
	if h.ConsentTTL() > h.Resume() {
		t.Error("a consent card outlives the resume window")
	}
	if !(h.StopBudget() < h.App.Verify() && h.App.Verify() < h.App.Chunk() && h.App.Settle() < h.App.Chunk()) {
		t.Error("a stop does not outrun a window check, or a check does not fit a typing call")
	}
	if h.Desktop.Idle() < h.Resume() || h.Task.Time() <= h.Resume() {
		t.Error("the nested desktop or the task ends inside the resume window")
	}
}

// A Hands model request waits as long as a guide read, so the two
// screen-reading paths give a slow route the same time.
func TestHandsModelRequestIsTheGuideRead(t *testing.T) {
	s := mustLoad(t)
	if s.Hands.ReadTimeoutS != s.Guide.ReadTimeoutS {
		t.Fatalf("hands.readTimeoutS = %v, guide.readTimeoutS = %v", s.Hands.ReadTimeoutS, s.Guide.ReadTimeoutS)
	}
}

// handsLeaves lists every hands.* value path in the embedded spec.
func handsLeaves(t *testing.T) map[string]string {
	t.Helper()
	flat, err := flattenJSON(embedded)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for p, v := range flat {
		if strings.HasPrefix(p, "hands.") {
			out[p] = v
		}
	}
	if len(out) != 41 {
		t.Fatalf("found %d hands values, want 41", len(out))
	}
	return out
}

func TestEveryHandsNumberRejectsANegative(t *testing.T) {
	leaves := handsLeaves(t)
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
	if n != 41 {
		t.Fatalf("checked %d hands numbers, want 41", n)
	}
}

func TestEveryHandsValueDriftBitesTheDoc(t *testing.T) {
	md, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatal(err)
	}
	if diffs, err := parity(md, embedded); err != nil || len(diffs) > 0 {
		t.Fatalf("doc and json disagree: %q %v", diffs, err)
	}
	leaves := handsLeaves(t)
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
	for _, row := range []struct{ line, path string }{
		{"| stopBudgetMs | 250 |\n", "hands.stopBudgetMs"},
		{"| typeChunkRunes | 40 |\n", "hands.app.typeChunkRunes"},
		{"| stopGraceS | 3 |\n", "hands.desktop.stopGraceS"},
		{"| hostedUsd | 2 |\n", "hands.task.hostedUsd"},
		{"| days | 30 |\n", "hands.retention.days"},
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

func TestHandsLoadAcceptsTheEdges(t *testing.T) {
	set := func(path, v string) []byte {
		return edited(t, func(r map[string]any) { setPath(r, path, json.Number(v)) })
	}
	for _, c := range []struct {
		name string
		data []byte
	}{
		{"no extra steps", set("hands.budget.extra", "0")},
		{"repeat limit at the budget", set("hands.repeatLimit", "15")},
		{"history as long as the budget", set("hands.historyCarry", "15")},
		{"card lapses with the resume window", set("hands.consentTtlS", "300")},
		{"model request ends with the resume window", set("hands.readTimeoutS", "300")},
		{"model request just outlasts an action", set("hands.readTimeoutS", "3.001")},
		{"fill as long as the page text", set("hands.page.maxFillRunes", "6000")},
		{"glow at its frame ceiling", set("hands.glow.fps", "30")},
		{"glow breath of two frames", set("hands.glow.periodS", "0.1667")},
		{"no geometry tolerance", set("hands.app.geometryTolPx", "0")},
		{"geometry tolerance at its ceiling", set("hands.app.geometryTolPx", "8")},
		{"one-rune chunks", set("hands.app.typeChunkRunes", "1")},
		{"type as long as a chunk", set("hands.app.maxTypeRunes", "40")},
		{"type as long as a fill", set("hands.app.maxTypeRunes", "2000")},
		{"check just outlasts the stop", set("hands.app.verifyMs", "251")},
		{"check just inside a typing call", set("hands.app.verifyMs", "4999")},
		{"keymap wait just inside a typing call", set("hands.app.settleMs", "4999")},
		{"typed read-back just outlasts the stop", set("hands.app.readbackMs", "251")},
		{"nested stop just inside a start", set("hands.desktop.stopGraceS", "19.9")},
		{"nested desktop idles with the resume window", set("hands.desktop.idleS", "300")},
		{"task turns as the step budget", set("hands.task.turns", "25")},
		{"task just outlasts the resume window", set("hands.task.ms", "300001")},
		{"queue one below the journal", set("hands.task.queueMax", "199")},
		{"hosted cap of a cent", set("hands.task.hostedUsd", "0.01")},
	} {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Parse(c.data); err != nil {
				t.Fatalf("refused: %v", err)
			}
		})
	}
}

func TestHandsLoadRejects(t *testing.T) {
	set := func(path, v string) []byte {
		return edited(t, func(r map[string]any) { setPath(r, path, json.Number(v)) })
	}
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"missing hands", edited(t, func(r map[string]any) { delete(r, "hands") }), "hands: missing key"},
		{"unknown hands key", edited(t, func(r map[string]any) { node(r, "hands")["pressSubmit"] = true }), "hands.pressSubmit: unknown key"},
		{"unknown budget key", edited(t, func(r map[string]any) { node(r, "hands.budget")["bonus"] = json.Number("5") }), "hands.budget.bonus: unknown key"},
		{"missing fill bound", edited(t, func(r map[string]any) { delete(node(r, "hands.page"), "maxFillRunes") }), "hands.page.maxFillRunes: missing key"},
		{"missing glow", edited(t, func(r map[string]any) { delete(node(r, "hands"), "glow") }), "hands.glow: missing key"},

		{"no steps", set("hands.budget.steps", "0"), "hands.budget.steps: must be at least 1"},
		{"fractional steps", set("hands.budget.steps", "15.5"), "hands.budget.steps: want an integer"},
		{"fractional extra", set("hands.budget.extra", "10.5"), "hands.budget.extra: want an integer"},

		{"no repeat limit", set("hands.repeatLimit", "0"), "hands.repeatLimit: must lie in [2, budget.steps]"},
		{"repeat halts on the first step", set("hands.repeatLimit", "1"), "hands.repeatLimit: must lie in [2, budget.steps]"},
		{"repeat limit past the budget", set("hands.repeatLimit", "16"), "hands.repeatLimit: must lie in [2, budget.steps]"},
		{"budget under the repeat limit", set("hands.budget.steps", "2"), "hands.repeatLimit: must lie in [2, budget.steps]"},
		{"fractional repeat limit", set("hands.repeatLimit", "3.5"), "hands.repeatLimit: want an integer"},

		{"no done rejections", set("hands.doneRejections", "0"), "hands.doneRejections: must lie in [1, budget.steps)"},
		{"rejections fill the budget", set("hands.doneRejections", "15"), "hands.doneRejections: must lie in [1, budget.steps)"},
		{"fractional rejections", set("hands.doneRejections", "2.5"), "hands.doneRejections: want an integer"},

		{"zero resume", set("hands.resumeS", "0"), "hands.resumeS: must be positive"},
		{"resume shorter than a card", set("hands.resumeS", "119"), "hands.consentTtlS: a waiting card must lapse within the resume window"},
		{"zero follow-up", set("hands.followUpS", "0"), "hands.followUpS: must be positive"},

		{"no history", set("hands.historyCarry", "0"), "hands.historyCarry: must lie in [1, budget.steps]"},
		{"history past the budget", set("hands.historyCarry", "16"), "hands.historyCarry: must lie in [1, budget.steps]"},
		{"fractional history", set("hands.historyCarry", "6.5"), "hands.historyCarry: want an integer"},

		{"zero card lifetime", set("hands.consentTtlS", "0"), "hands.consentTtlS: must be positive"},
		{"card outlives the resume window", set("hands.consentTtlS", "301"), "hands.consentTtlS: a waiting card must lapse within the resume window"},

		{"zero model request", set("hands.readTimeoutS", "0"), "hands.readTimeoutS: must be positive"},
		{"model request as short as an action", set("hands.readTimeoutS", "3"), "hands.readTimeoutS: want actMs < readTimeoutS·1000 and readTimeoutS <= resumeS"},
		{"model request outlives the resume window", set("hands.readTimeoutS", "301"), "hands.readTimeoutS: want actMs < readTimeoutS·1000 and readTimeoutS <= resumeS"},
		{"action outlasts a model request", set("hands.actMs", "75000"), "hands.readTimeoutS: want actMs < readTimeoutS·1000 and readTimeoutS <= resumeS"},
		{"resume window inside a model request", set("hands.resumeS", "74"), "hands.readTimeoutS: want actMs < readTimeoutS·1000 and readTimeoutS <= resumeS"},

		{"zero read-back", set("hands.readbackMs", "0"), "hands.readbackMs: must be at least 1"},
		{"read-back as long as an action", set("hands.readbackMs", "3000"), "hands.stopBudgetMs: want stopBudgetMs < readbackMs < actMs"},
		{"read-back within the stop", set("hands.readbackMs", "250"), "hands.stopBudgetMs: want stopBudgetMs < readbackMs < actMs"},
		{"fractional read-back", set("hands.readbackMs", "1500.5"), "hands.readbackMs: want an integer"},
		{"zero action", set("hands.actMs", "0"), "hands.actMs: must be at least 1"},
		{"action shorter than its read-back", set("hands.actMs", "1500"), "hands.stopBudgetMs: want stopBudgetMs < readbackMs < actMs"},
		{"fractional action", set("hands.actMs", "3000.5"), "hands.actMs: want an integer"},
		{"zero stop budget", set("hands.stopBudgetMs", "0"), "hands.stopBudgetMs: must be at least 1"},
		{"stop as slow as a read-back", set("hands.stopBudgetMs", "1500"), "hands.stopBudgetMs: want stopBudgetMs < readbackMs < actMs"},
		{"fractional stop budget", set("hands.stopBudgetMs", "250.5"), "hands.stopBudgetMs: want an integer"},

		{"no controls", set("hands.page.maxControls", "0"), "hands.page.maxControls: must be at least 1"},
		{"fractional controls", set("hands.page.maxControls", "200.5"), "hands.page.maxControls: want an integer"},
		{"no name", set("hands.page.maxNameRunes", "0"), "hands.page.maxNameRunes: must be at least 1"},
		{"name as long as the page text", set("hands.page.maxNameRunes", "6000"), "hands.page.maxNameRunes: a control name must be shorter than the page text bound"},
		{"fractional name", set("hands.page.maxNameRunes", "80.5"), "hands.page.maxNameRunes: want an integer"},
		{"no page text", set("hands.page.maxTextRunes", "0"), "hands.page.maxTextRunes: must be at least 1"},
		{"page text shorter than a fill", set("hands.page.maxTextRunes", "1999"), "hands.page.maxFillRunes: a fill must fit in the page text the read-back sees"},
		{"fractional page text", set("hands.page.maxTextRunes", "6000.5"), "hands.page.maxTextRunes: want an integer"},
		{"no fill", set("hands.page.maxFillRunes", "0"), "hands.page.maxFillRunes: must be at least 1"},
		{"fill past the page text", set("hands.page.maxFillRunes", "6001"), "hands.page.maxFillRunes: a fill must fit in the page text the read-back sees"},
		{"fractional fill", set("hands.page.maxFillRunes", "2000.5"), "hands.page.maxFillRunes: want an integer"},

		{"glow never repaints", set("hands.glow.fps", "0"), "hands.glow.fps: must lie in [1, 30]"},
		{"glow at full rate", set("hands.glow.fps", "31"), "hands.glow.fps: must lie in [1, 30]"},
		{"fractional glow rate", set("hands.glow.fps", "12.5"), "hands.glow.fps: want an integer"},
		{"glow low dark", set("hands.glow.low", "0"), "hands.glow: want 0 < low < high <= 1"},
		{"glow low at its high", set("hands.glow.low", "1"), "hands.glow: want 0 < low < high <= 1"},
		{"glow high past full", set("hands.glow.high", "1.1"), "hands.glow: want 0 < low < high <= 1"},
		{"glow high under its low", set("hands.glow.high", "0.7"), "hands.glow: want 0 < low < high <= 1"},
		{"zero glow breath", set("hands.glow.periodS", "0"), "hands.glow.periodS: must be positive"},
		{"glow breath under two frames", set("hands.glow.periodS", "0.1"), "hands.glow.periodS: one breath must span at least two frames"},
		{"no glow strip", set("hands.glow.stripPx", "0"), "hands.glow.stripPx: must be at least 1"},
		{"fractional glow strip", set("hands.glow.stripPx", "130.5"), "hands.glow.stripPx: want an integer"},
		{"waiting glow gone", set("hands.glow.waitingFactor", "0"), "hands.glow.waitingFactor: must lie in (0, 1)"},
		{"waiting glow at full strength", set("hands.glow.waitingFactor", "1"), "hands.glow.waitingFactor: must lie in (0, 1)"},

		{"empty ledger", set("hands.ledger.maxRows", "0"), "hands.ledger.maxRows: must be at least 1"},
		{"fractional ledger", set("hands.ledger.maxRows", "500.5"), "hands.ledger.maxRows: want an integer"},

		{"missing app", edited(t, func(r map[string]any) { delete(node(r, "hands"), "app") }), "hands.app: missing key"},
		{"unknown app key", edited(t, func(r map[string]any) { node(r, "hands.app")["drag"] = true }), "hands.app.drag: unknown key"},
		{"missing chunk bound", edited(t, func(r map[string]any) { delete(node(r, "hands.app"), "typeChunkRunes") }), "hands.app.typeChunkRunes: missing key"},
		{"geometry tolerance past its ceiling", set("hands.app.geometryTolPx", "9"), "hands.app.geometryTolPx: must lie in [0, 8]"},
		{"fractional geometry tolerance", set("hands.app.geometryTolPx", "2.5"), "hands.app.geometryTolPx: want an integer"},
		{"zero window check", set("hands.app.verifyMs", "0"), "hands.app.verifyMs: must be at least 1"},
		{"window check within the stop", set("hands.app.verifyMs", "250"), "hands.app.verifyMs: want stopBudgetMs < verifyMs < chunkMs"},
		{"window check as long as a typing call", set("hands.app.verifyMs", "5000"), "hands.app.verifyMs: want stopBudgetMs < verifyMs < chunkMs"},
		{"fractional window check", set("hands.app.verifyMs", "500.5"), "hands.app.verifyMs: want an integer"},
		{"zero keymap wait", set("hands.app.settleMs", "0"), "hands.app.settleMs: must be at least 1"},
		{"keymap wait fills a typing call", set("hands.app.settleMs", "5000"), "hands.app.settleMs: a typing call must outlast its keymap wait"},
		{"fractional keymap wait", set("hands.app.settleMs", "50.5"), "hands.app.settleMs: want an integer"},
		{"zero typing call", set("hands.app.chunkMs", "0"), "hands.app.chunkMs: must be at least 1"},
		{"typing call as short as a check", set("hands.app.chunkMs", "500"), "hands.app.verifyMs: want stopBudgetMs < verifyMs < chunkMs"},
		{"fractional typing call", set("hands.app.chunkMs", "5000.5"), "hands.app.chunkMs: want an integer"},
		{"zero typed read-back", set("hands.app.readbackMs", "0"), "hands.app.readbackMs: must be at least 1"},
		{"typed read-back within the stop", set("hands.app.readbackMs", "250"), "hands.app.readbackMs: a stop must outrun a typed read-back"},
		{"fractional typed read-back", set("hands.app.readbackMs", "4000.5"), "hands.app.readbackMs: want an integer"},
		{"no chunk", set("hands.app.typeChunkRunes", "0"), "hands.app.typeChunkRunes: must lie in [1, 40]"},
		{"chunk past the contract", set("hands.app.typeChunkRunes", "41"), "hands.app.typeChunkRunes: must lie in [1, 40]"},
		{"fractional chunk", set("hands.app.typeChunkRunes", "40.5"), "hands.app.typeChunkRunes: want an integer"},
		{"no type", set("hands.app.maxTypeRunes", "0"), "hands.app.maxTypeRunes: must be at least 1"},
		{"type shorter than a chunk", set("hands.app.maxTypeRunes", "39"), "hands.app.maxTypeRunes: want typeChunkRunes <= maxTypeRunes <= page.maxFillRunes"},
		{"type past a fill", set("hands.app.maxTypeRunes", "2001"), "hands.app.maxTypeRunes: want typeChunkRunes <= maxTypeRunes <= page.maxFillRunes"},
		{"fill shorter than a type", set("hands.page.maxFillRunes", "199"), "hands.app.maxTypeRunes: want typeChunkRunes <= maxTypeRunes <= page.maxFillRunes"},
		{"fractional type", set("hands.app.maxTypeRunes", "200.5"), "hands.app.maxTypeRunes: want an integer"},
		{"stop as slow as a window check", set("hands.stopBudgetMs", "500"), "hands.app.verifyMs: want stopBudgetMs < verifyMs < chunkMs"},

		{"missing desktop", edited(t, func(r map[string]any) { delete(node(r, "hands"), "desktop") }), "hands.desktop: missing key"},
		{"unknown desktop key", edited(t, func(r map[string]any) { node(r, "hands.desktop")["liveFallback"] = true }), "hands.desktop.liveFallback: unknown key"},
		{"no nested width", set("hands.desktop.widthPx", "0"), "hands.desktop.widthPx: must be at least 1"},
		{"fractional nested width", set("hands.desktop.widthPx", "1280.5"), "hands.desktop.widthPx: want an integer"},
		{"no nested height", set("hands.desktop.heightPx", "0"), "hands.desktop.heightPx: must be at least 1"},
		{"fractional nested height", set("hands.desktop.heightPx", "800.5"), "hands.desktop.heightPx: want an integer"},
		{"zero nested ready", set("hands.desktop.readyS", "0"), "hands.desktop.readyS: must be positive"},
		{"nested start shorter than its stop", set("hands.desktop.readyS", "3"), "hands.desktop.stopGraceS: a stop must end the nested desktop sooner than a start may take"},
		{"zero nested stop grace", set("hands.desktop.stopGraceS", "0"), "hands.desktop.stopGraceS: must be positive"},
		{"nested stop as slow as a start", set("hands.desktop.stopGraceS", "20"), "hands.desktop.stopGraceS: a stop must end the nested desktop sooner than a start may take"},
		{"zero nested idle", set("hands.desktop.idleS", "0"), "hands.desktop.idleS: must be positive"},
		{"nested desktop idles out inside the resume window", set("hands.desktop.idleS", "299"), "hands.desktop.idleS: the nested desktop must outlive the resume window"},

		{"missing task", edited(t, func(r map[string]any) { delete(node(r, "hands"), "task") }), "hands.task: missing key"},
		{"unknown task key", edited(t, func(r map[string]any) { node(r, "hands.task")["retries"] = json.Number("2") }), "hands.task.retries: unknown key"},
		{"no turns", set("hands.task.turns", "0"), "hands.task.turns: must be at least 1"},
		{"turns under the step budget", set("hands.task.turns", "24"), "hands.task.turns: a task must allow one full step budget"},
		{"step budget past the turns", set("hands.budget.extra", "26"), "hands.task.turns: a task must allow one full step budget"},
		{"fractional turns", set("hands.task.turns", "40.5"), "hands.task.turns: want an integer"},
		{"no task time", set("hands.task.ms", "0"), "hands.task.ms: must be at least 1"},
		{"task ends with the resume window", set("hands.task.ms", "300000"), "hands.task.ms: a task must outlast the resume window"},
		{"fractional task time", set("hands.task.ms", "600000.5"), "hands.task.ms: want an integer"},
		{"zero hosted cap", set("hands.task.hostedUsd", "0"), "hands.task.hostedUsd: must be positive"},
		{"empty queue", set("hands.task.queueMax", "0"), "hands.task.queueMax: must be at least 1"},
		{"queue fills the journal", set("hands.task.queueMax", "200"), "hands.task.queueMax: the journal must hold the running task and a full queue"},
		{"fractional queue", set("hands.task.queueMax", "1.5"), "hands.task.queueMax: want an integer"},

		{"missing retention", edited(t, func(r map[string]any) { delete(node(r, "hands"), "retention") }), "hands.retention: missing key"},
		{"unknown retention key", edited(t, func(r map[string]any) { node(r, "hands.retention")["titles"] = true }), "hands.retention.titles: unknown key"},
		{"no retention days", set("hands.retention.days", "0"), "hands.retention.days: must be at least 1"},
		{"fractional retention days", set("hands.retention.days", "30.5"), "hands.retention.days: want an integer"},
		{"no retention rows", set("hands.retention.rows", "0"), "hands.retention.rows: must be at least 1"},
		{"journal no larger than the queue", set("hands.retention.rows", "1"), "hands.task.queueMax: the journal must hold the running task and a full queue"},
		{"fractional retention rows", set("hands.retention.rows", "200.5"), "hands.retention.rows: want an integer"},
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
