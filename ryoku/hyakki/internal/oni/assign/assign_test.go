package assign

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/screenask"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
)

func gate(t *testing.T) GateConfig {
	t.Helper()
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	return Gate(sp.Assignment.Gate)
}

func gray(v uint8) []uint8 {
	out := make([]uint8, 48*27)
	for i := range out {
		out[i] = v
	}
	return out
}

// moved is a fingerprint well past the still threshold from gray(100).
func moved() []uint8 { return gray(140) }

func words(s string) []screenask.Word {
	var out []screenask.Word
	for row, line := range strings.Split(s, "\n") {
		for _, text := range strings.Fields(line) {
			out = append(out, screenask.Word{Text: text, Box: feature.Box{X: 0.1, Y: 0.1 + 0.1*float64(row), W: 0.03, H: 0.03}})
		}
	}
	return out
}

func TestGate(t *testing.T) {
	g := gate(t)
	problem := View{Gray: gray(100), Words: words("Problem 4 Solve for x: 3x + 5 = 20 Answer:")}
	for _, c := range []struct {
		name string
		cur  View
		new  bool
		by   string
	}{
		{"a cursor or caret moving", View{Gray: gray(101), Words: problem.Words}, false, "pixels"},
		{"identical pixels with different OCR", View{Gray: gray(100), Words: words("Question 9 Calculate 7 + 8 = 15")}, false, "pixels"},
		{"identical pixels with missing OCR", View{Gray: gray(100)}, false, "pixels"},
		{"small answer addition", View{Gray: gray(101), Words: words("Problem 4 Solve for x: 3x + 5 = 20 Answer: x = 5")}, false, "pixels"},
		{"small clock addition", View{Gray: gray(101), Words: words("10:42 Problem 4 Solve for x: 3x + 5 = 20 Answer:")}, false, "pixels"},
		{"small problem number change", View{Gray: gray(101), Words: words("Problem 5 Solve for x: 3x + 5 = 20 Answer:")}, true, "critical"},
		{"small math change", View{Gray: gray(101), Words: words("Problem 4 Solve for x: 3x + 6 = 21 Answer:")}, true, "critical"},
		{"small noncritical word loss", View{Gray: gray(101), Words: words("Problem 4 3x + 5 = 20")}, false, "pixels"},
		{"typing the answer adds words", View{Gray: moved(), Words: words("Problem 4 Solve for x: 3x + 5 = 20 Answer: x = 5")}, false, "words"},
		{"a clock ticking", View{Gray: moved(), Words: words("10:42 Problem 4 Solve for x: 3x + 5 = 20 Answer:")}, false, "words"},
		{"the next problem", View{Gray: moved(), Words: words("Problem 5 Factor completely: x^2 - 9 Answer:")}, true, "words"},
		{"a new problem number only", View{Gray: moved(), Words: words("Problem 5 Solve for x: 3x + 5 = 20 Answer:")}, true, "critical"},
		{"the numbers changed", View{Gray: moved(), Words: words("Problem 4 Solve for x: 7x - 2 = 20 Answer:")}, true, "critical"},
		{"a different size cannot be compared", View{Gray: gray(100)[:10], Words: problem.Words}, true, "pixels"},
	} {
		v := g.NewProblem(problem, c.cur)
		if v.New != c.new || v.By != c.by {
			t.Errorf("%s: %+v", c.name, v)
		}
	}
	clock := View{Gray: gray(100), Words: words("10:41 Problem 4 Solve for x: 3x + 5 = 20")}
	for _, pixels := range [][]uint8{gray(100), gray(101), moved()} {
		if v := g.NewProblem(clock, View{Gray: pixels, Words: words("10:42 Problem 4 Solve for x: 3x + 5 = 20")}); v.New {
			t.Fatalf("a clock tick was a new problem: %+v", v)
		}
	}
}

func TestGateSmallCriticalTimeChanges(t *testing.T) {
	g := gate(t)
	before := View{Gray: gray(100), Words: words("Problem 4 A train departs at 10:41 and arrives at 11:15. Find the journey time.")}
	after := View{Gray: gray(101), Words: words("Problem 4 A train departs at 10:43 and arrives at 11:20. Find the journey time.")}
	if verdict := g.NewProblem(before, after); !verdict.New || verdict.By != "critical" {
		t.Fatalf("changed times in the problem were ignored: %+v", verdict)
	}
}

func TestGateCriticalEvidenceSurvivesOCRWordGrouping(t *testing.T) {
	g := gate(t)
	for _, tc := range []struct {
		name, before, after string
		new                 bool
	}{
		{"two compact constants changed", "Problem 4 Solvex+2=5", "Problem 4 Solvex+3=7", true},
		{"same equation grouped differently", "Problem 4 Solve x +2 =5", "Problem 4 Solvex+2=5 Answer: 3", false},
		{"two compact operators changed", "Problem 4 a+b*c", "Problem 4 a-b/c", true},
		{"one scalar changed", "Problem 4 x+10=20", "Problem 4 x+11=20", false},
		{"two decimal constants changed", "Problem 4 x+0.25=2.50", "Problem 4 x+0.50=3.50", true},
		{"two equal constants changed", "Problem 4 2+2=4", "Problem 4 3+3=4", true},
		{"a repeated constant and total changed", "Problem 4 2+2=4", "Problem 4 3+2=5", true},
		{"clock crosses an hour", "10:59 Problem 4 x+2=5", "11:00 Problem 4 x+2=5", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := View{Gray: gray(100), Words: words(tc.before)}
			after := View{Gray: gray(101), Words: words(tc.after)}
			if got := g.NewProblem(before, after); got.New != tc.new {
				t.Fatalf("%q -> %q: %+v; new=%v", tc.before, tc.after, got, tc.new)
			}
		})
	}
}

func TestCriticalReplacementsStayOnTheirTextRow(t *testing.T) {
	g := gate(t)
	before := View{Gray: gray(100), Words: words("Problem: 2=5\nSolvex+2=5")}
	for _, tc := range []struct {
		name, text string
		new        bool
	}{
		{"lost duplicate and new answer", "Problem: clo\nSolvex+2=5\nAnswer: 3", false},
		{"lost duplicate and multi-number answer", "Problem: clo\nSolvex+2=5\nAnswer: 3+4=7", false},
		{"partial critical replacement", "Problem:\nSolve x +3", true},
		{"missing OCR without replacement", "Problem:\n\nAnswer: 3", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			after := View{Gray: gray(101), Words: words(tc.text)}
			if got := g.NewProblem(before, after); got.New != tc.new {
				t.Fatalf("%+v; new=%v", got, tc.new)
			}
		})
	}
}

func TestParseFastAndChecker(t *testing.T) {
	steps := StepLimits{Min: 3, Max: 8}
	if s, err := ParseFast(`{"found": true, "answer": "x = 5"}`); err != nil || !s.Found || s.Answer != "x = 5" {
		t.Fatalf("fast: %+v %v", s, err)
	}
	if s, err := ParseFast("```json\n{\"found\": false, \"answer\": \"\"}\n```"); err != nil || s.Found {
		t.Fatalf("fenced fast: %+v %v", s, err)
	}
	good := `{"found":true,"problem":"3x+5=20","steps":["subtract 5","3x = 15","divide by 3"],"answer":"x = 5","check":"3*5+5=20","confidence":0.9}`
	if s, err := ParseChecker(good, steps); err != nil || len(s.Steps) != 3 || s.Answer != "x = 5" {
		t.Fatalf("checker: %+v %v", s, err)
	}
	for name, c := range map[string]struct {
		raw  string
		want error
	}{
		"invented field": {`{"found":true,"problem":"p","steps":["a","b","c"],"answer":"5","check":"c","confidence":0.9,"tool":"run"}`, ErrShape},
		"two steps":      {`{"found":true,"problem":"p","steps":["a","b"],"answer":"5","check":"c","confidence":0.9}`, ErrSteps},
		"nine steps":     {`{"found":true,"problem":"p","steps":["a","b","c","d","e","f","g","h","i"],"answer":"5","check":"c","confidence":0.9}`, ErrSteps},
		"an empty step":  {`{"found":true,"problem":"p","steps":["a","","c"],"answer":"5","check":"c","confidence":0.9}`, ErrShape},
		"a card number":  {`{"found":true,"problem":"p","steps":["a","pay with 4111 1111 1111 1111","c"],"answer":"5","check":"c","confidence":0.9}`, ErrPrivate},
		"a key":          {`{"found":true,"problem":"p","steps":["a","b","c"],"answer":"sk-ant-api03-AAAAbbbbCCCCddddEEEEffff","check":"c","confidence":0.9}`, ErrPrivate},
		"no confidence":  {`{"found":true,"problem":"p","steps":["a","b","c"],"answer":"5","check":"c"}`, ErrShape},
		"bad confidence": {`{"found":true,"problem":"p","steps":["a","b","c"],"answer":"5","check":"c","confidence":2}`, ErrShape},
		"no answer":      {`{"found":true,"problem":"p","steps":["a","b","c"],"answer":"","check":"c","confidence":0.5}`, ErrShape},
		"trailing":       {good + ` {"found":false}`, ErrShape},
		"prose":          {`The answer is 5.`, ErrShape},
	} {
		if _, err := ParseChecker(c.raw, steps); !errors.Is(err, c.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := ParseFast(`{"found":true,"answer":"5","steps":[]}`); !errors.Is(err, ErrShape) {
		t.Fatalf("a fast reply with steps: %v", err)
	}
}

func TestSame(t *testing.T) {
	for _, c := range []struct {
		a, b string
		same bool
	}{
		{"x = 5", "5", true},
		{"X=5.", "x = 5", true},
		{"0.5", "1/2", true},
		{"50%", "0.5", true},
		{"1,000", "1000", true},
		{"5", "6", false},
		{"(x-3)(x+3)", "(x+3)(x-3)", false},
		{"", "", false},
	} {
		if Same(c.a, c.b) != c.same {
			t.Errorf("Same(%q, %q) != %v", c.a, c.b, c.same)
		}
	}
	if !JudgeSame(" Same.") || JudgeSame("different") || JudgeSame("same, I think") {
		t.Fatal("JudgeSame")
	}
}

func TestCacheRestoresASeenView(t *testing.T) {
	g := gate(t)
	c := NewCache(2)
	v1 := View{Gray: gray(10), Words: words("Problem 1 2+2")}
	v2 := View{Gray: gray(80), Words: words("Problem 2 3+3")}
	v3 := View{Gray: gray(160), Words: words("Problem 3 4+4")}
	for _, v := range []View{v1, v2, v3} {
		c.Put(Solved{View: v, Solve: Solve{Found: true, Answer: v.Words[2].Text}})
	}
	if c.Len() != 2 {
		t.Fatalf("kept %d", c.Len())
	}
	if _, _, ok := c.Find(g, v1); ok {
		t.Fatal("the oldest view outlived the cache")
	}
	if s, i, ok := c.Find(g, View{Gray: gray(81), Words: v2.Words}); !ok || i != 0 || s.Solve.Answer != "3+3" {
		t.Fatalf("find: %+v %d %v", s, i, ok)
	}
}

func TestLedgerCapsHostedSpendAndKeepsTheDay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "assignment.json")
	loc := time.FixedZone("here", 3600)
	now := time.Date(2026, 11, 23, 22, 30, 0, 0, time.UTC) // 23:30 here
	l := NewLedger(path, Caps{Attempts: 2, Tokens: 100}, 3, loc)
	if err := l.Allow(now, false); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		if err := l.Allow(now, true); err != nil {
			t.Fatal("a local solve was capped")
		}
		_ = l.Attempt(now, true)
		_ = l.Record(protocol.OniAssignmentRow{At: now, Lane: protocol.OniAssignmentLaneFast, Route: "local/m", Input: 500, Output: 500, Outcome: "ok"}, true)
	}
	if err := l.Allow(now, false); err != nil {
		t.Fatalf("local tokens counted against the hosted cap: %v", err)
	}
	_ = l.Attempt(now, false)
	_ = l.Record(protocol.OniAssignmentRow{At: now, Lane: protocol.OniAssignmentLaneFast, Route: "x/y", Input: 30, Output: 10, Estimated: true, Outcome: "ok"}, false)
	_ = l.Attempt(now, false)
	if err := l.Allow(now, false); !errors.Is(err, ErrAttempts) {
		t.Fatalf("attempts cap: %v", err)
	}
	day := l.Day(now)
	if day.Day != "2026-11-23" || day.Attempts != 7 || day.Tokens != 5040 || !day.Estimated || day.Unmetered || day.AttemptsCap != 2 || day.TokensCap != 100 {
		t.Fatalf("day %+v", day)
	}
	if rows := l.Rows(now); len(rows) != 3 || rows[0].Route != "x/y" {
		t.Fatalf("rows not bounded or not newest first: %+v", rows)
	}
	// a restart keeps the counts, never the rows
	again := NewLedger(path, Caps{Attempts: 2, Tokens: 100}, 3, loc)
	if err := again.Allow(now, false); !errors.Is(err, ErrAttempts) || len(again.Rows(now)) != 0 {
		t.Fatalf("a restart reset the cap: %v", err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("file: %v %v", info, err)
	}
	b, _ := os.ReadFile(path)
	if strings.Contains(string(b), "x/y") || strings.Contains(string(b), "fast") {
		t.Fatalf("a row reached the disk: %s", b)
	}
	// the local day rolls over at local midnight
	next := now.Add(31 * time.Minute)
	if err := again.Allow(next, false); err != nil || again.Day(next).Attempts != 0 {
		t.Fatalf("the day did not roll: %v %+v", err, again.Day(next))
	}
}

func TestLedgerTokenCapAndUnmetered(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	l := NewLedger("", Caps{Attempts: 60, Tokens: 100}, 10, time.UTC)
	_ = l.Attempt(now, false)
	_ = l.Record(protocol.OniAssignmentRow{At: now, Route: "x/y", Input: 80, Output: 20}, false)
	if err := l.Allow(now, false); !errors.Is(err, ErrTokens) {
		t.Fatalf("token cap: %v", err)
	}
	u := NewLedger("", Caps{Attempts: 60, Tokens: 100000}, 10, time.UTC)
	_ = u.Attempt(now, false)
	_ = u.Record(protocol.OniAssignmentRow{At: now, Route: "x/y", Unmetered: true}, false)
	if err := u.Allow(now, false); !errors.Is(err, ErrUnmetered) || !u.Day(now).Unmetered {
		t.Fatalf("unmetered hosted usage was treated as free: %v", err)
	}
	if err := u.Allow(now, true); err != nil {
		t.Fatal("unmetered hosted usage stopped a local solve")
	}
}
