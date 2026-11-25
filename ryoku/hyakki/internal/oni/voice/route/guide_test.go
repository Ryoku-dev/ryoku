package route

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// fakeGuide records what the router asked of the running guide.
type fakeGuide struct {
	active  bool
	nexts   []Press
	circles int
	stops   int
	err     error
}

func (g *fakeGuide) Active() bool { return g.active }

func (g *fakeGuide) Next(p Press) error {
	g.nexts = append(g.nexts, p)
	return g.err
}

func (g *fakeGuide) Circle() error {
	g.circles++
	return g.err
}

func (g *fakeGuide) Stop() { g.stops++ }

// A spoken next moves a running guide on only from a press; wake and open
// mic are told to press instead and the guide is untouched. Circle and a
// dismissal work from any source; with no guide running neither next nor
// circle means anything special.
func TestRouteGuideNextOnlyFromAPressStopAndCircleFromAny(t *testing.T) {
	g := &fakeGuide{active: true}
	r, tn := newRouter(t, Config{Name: "Oni", Guide: g})
	id := tn.Barge()
	route := func(text string, s Source) Decision {
		return r.Route(Utterance{Text: text, Source: s, Turn: id, At: t0})
	}
	for _, s := range []Source{Chord, Toggle} {
		for _, text := range []string{"next", "Done.", "Oni, next please", "okay, next step", "I did it", "what's next"} {
			n := len(g.nexts)
			if d := route(text, s); d.Kind != GuideNexted || d.Line != "" || d.Ask != nil {
				t.Fatalf("%s %q = %+v", s, text, d)
			}
			if len(g.nexts) != n+1 || !g.nexts[n].Valid() || g.nexts[n].Source() != s || g.nexts[n].Turn() != id {
				t.Fatalf("%s %q gave the guide %+v", s, text, g.nexts)
			}
		}
	}
	for _, s := range []Source{Wake, Open} {
		n := len(g.nexts)
		if d := route("next", s); d.Kind != NeedsPress || d.Line != NextNeedsPressLine {
			t.Fatalf("%s next = %+v", s, d)
		}
		if d := route("done", s); d.Kind != NeedsPress {
			t.Fatalf("%s done = %+v", s, d)
		}
		if len(g.nexts) != n {
			t.Fatalf("%s speech moved the guide on", s)
		}
	}
	for _, s := range []Source{Chord, Toggle, Wake, Open} {
		c, st := g.circles, g.stops
		if d := route("circle it", s); d.Kind != GuideCircled {
			t.Fatalf("%s circle = %+v", s, d)
		}
		if d := route("Stop.", s); d.Kind != Dismissed {
			t.Fatalf("%s stop = %+v", s, d)
		}
		if g.circles != c+1 || g.stops != st+1 {
			t.Fatalf("%s: circles %d stops %d", s, g.circles, g.stops)
		}
	}

	// the guide went away between the check and the call
	g.err = errors.New("gone")
	if d := route("next", Chord); d.Kind != Dropped || d.Why != WhyNoGuide {
		t.Fatalf("next for a gone guide = %+v", d)
	}
	if d := route("circle it", Wake); d.Kind != Dropped || d.Why != WhyNoGuide {
		t.Fatalf("circle for a gone guide = %+v", d)
	}
	g.err = nil

	// no guide: the words are an ordinary question, and a dismissal still
	// tells the guide slot to stop
	g.active = false
	n, c := len(g.nexts), g.circles
	if d := route("next", Chord); d.Kind != Asked || d.Ask == nil || d.Ask.Question != "next" {
		t.Fatalf("next with no guide = %+v", d)
	}
	if d := route("circle it", Chord); d.Kind != Asked {
		t.Fatalf("circle with no guide = %+v", d)
	}
	if len(g.nexts) != n || g.circles != c {
		t.Fatal("a guide that is not running was moved")
	}
	st := g.stops
	if d := route("never mind", Open); d.Kind != Dismissed || g.stops != st+1 {
		t.Fatalf("dismissal = %+v, stops %d", d, g.stops)
	}
}

// With Oni perched and guides on, a question about doing something on
// screen starts a guide toward what was said, from any source; otherwise,
// or for other questions, it is an Ask as before.
func TestRouteScreenQuestionStartsAGuideOnlyWhenPerched(t *testing.T) {
	perched := true
	r, tn := newRouter(t, Config{Name: "Oni", Lines: Lines{Ack: []string{"On it."}}, Guide: &fakeGuide{}, Perched: func() bool { return perched }})
	id := tn.Barge()
	route := func(text string, s Source) Decision {
		return r.Route(Utterance{Text: text, Source: s, Turn: id, At: t0})
	}
	screen := []string{
		"How can I save this file?", "Oni, where do I find the export button", "how can we change the font",
		"where is the settings menu", "show me how to print", "walk me through sharing this", "what do I click",
		"which button do I press to undo", "how to add a page",
	}
	for _, s := range []Source{Chord, Toggle, Wake, Open} {
		for _, text := range screen {
			d := route(text, s)
			if d.Kind != Guided || d.Ask != nil || d.Goal != strings.TrimSpace(text) {
				t.Fatalf("%s %q = %+v", s, text, d)
			}
		}
	}
	for _, text := range []string{"what is the weather like", "tell me a joke", "how are you", "what time is it", "next"} {
		if d := route(text, Chord); d.Kind == Guided {
			t.Fatalf("%q started a guide: %+v", text, d)
		}
	}
	long := "how do I " + strings.Repeat("save ", 100)
	if d := route(long, Chord); d.Kind != Guided || len([]rune(d.Goal)) > protocol.OniGuideGoalMax || !strings.HasPrefix(d.Goal, "how do I save") {
		t.Fatalf("a long question = %d runes", len([]rune(d.Goal)))
	}
	perched = false
	if d := route("How can I save this file?", Chord); d.Kind != Asked || d.Ask == nil || d.Ask.Question != "How can I save this file?" {
		t.Fatalf("not perched = %+v", d)
	}
	r2, tn2 := newRouter(t, Config{})
	if d := r2.Route(Utterance{Text: "how can I save this file", Source: Chord, Turn: tn2.Barge(), At: t0}); d.Kind != Asked {
		t.Fatalf("no perch check = %+v", d)
	}
}

// What is said of a guide step keeps the voice discipline: its first
// sentence when that fits, else its opening, never ending on a joining
// word.
func TestLeadSaysAStepsOpeningWithinTheDiscipline(t *testing.T) {
	cases := map[string]string{
		"Open the File menu.":                               "Open the File menu.",
		"Press Save. Then wait for the dialog.":             "Press Save.",
		"Open the File menu at the top left of the window.": "Open the File menu.",
		"Click Save in the toolbar to keep your changes.":   "Click Save in the toolbar.",
		"Type your name in the box beside Account.":         "Type your name.",
		"Choose Export, then pick a folder for the copy.":   "Choose Export.",
		"Use the spot I am pointing at.":                    "Use the spot.",
		"":                                                  "",
		"The the the the the the the.":                      "",
	}
	for step, want := range cases {
		got := Lead(step)
		if got != want {
			t.Errorf("Lead(%q) = %q, want %q", step, got, want)
		}
		if got != "" && !Fits(got) {
			t.Errorf("Lead(%q) = %q breaks the discipline", step, got)
		}
	}
	if !slices.Contains(leadTails, "the") {
		t.Fatal("the tails lost their articles")
	}
}
