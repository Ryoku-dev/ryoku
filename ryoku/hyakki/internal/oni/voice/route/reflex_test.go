package route

import (
	"testing"

	"ryoku-rashin/internal/oni/reflex"
)

// planOf is a Reflex slot that plans one action for every utterance and
// records what it heard.
type planOf struct {
	intent reflex.Intent
	heard  []string
	press  []Press
}

func (s *planOf) slot(u Utterance, p Press) (Decision, bool) {
	s.heard = append(s.heard, u.Text)
	s.press = append(s.press, p)
	plan := reflex.Plan{Actions: []reflex.Action{{Intent: s.intent, Confidence: 1}}, Line: "Done."}
	return Decision{Plan: &plan, Line: plan.Line}, true
}

// The slot comes after the local answers, the card answer, the guide and
// small talk, and before the slow path.
func TestReflexComesAfterGuideAndSmallTalk(t *testing.T) {
	s := &planOf{intent: reflex.Mute}
	g := &fakeGuide{active: true}
	r, tn := newRouter(t, Config{Name: "Oni", Reflex: s.slot, Consent: &recordingConsent{}, Guide: g, Perched: func() bool { return true }})
	id := tn.Barge()
	cases := []struct {
		say  string
		want Kind
	}{
		{"what time is it", Answered},
		{"yes", ConsentAnswered},
		{"next", GuideNexted},
		{"hello", Answered},
		{"thank you", Answered},
		{"how do i export this page", Guided},
		{"mute", Reflexed},
	}
	for _, c := range cases {
		if d := r.Route(Utterance{Text: c.say, Source: Chord, Turn: id, At: t0}); d.Kind != c.want {
			t.Errorf("%q = %v, want %v", c.say, d.Kind, c.want)
		}
	}
	if len(s.heard) != 1 || s.heard[0] != "mute" {
		t.Fatalf("the slot heard %q", s.heard)
	}
}

// A pressed utterance reaches the slot with a valid Press, and the
// decision carries it and the plan.
func TestReflexFromAPressCarriesThePress(t *testing.T) {
	s := &planOf{intent: reflex.RememberNote}
	r, tn := newRouter(t, Config{Name: "Oni", Reflex: s.slot})
	for _, src := range []Source{Chord, Toggle} {
		id := tn.Barge()
		d := r.Route(Utterance{Text: "remember that the exam is friday", Source: src, Turn: id, At: t0})
		if d.Kind != Reflexed || !d.Press.Valid() || d.Press.Source() != src || d.Plan == nil || d.Plan.Actions[0].Intent != reflex.RememberNote {
			t.Fatalf("%v: %+v", src, d)
		}
	}
	for _, p := range s.press {
		if !p.Valid() {
			t.Fatalf("the slot got an invalid press from a pressed source: %+v", p)
		}
	}
}

// Wake and addressed open-mic speech never carry a Press-only intent to
// the daemon: the plan becomes NeedsPress, and the handler below fails the
// test if a Reflexed decision with one ever comes out.
func TestWakeAndOpenNeverReachAPressOnlyIntent(t *testing.T) {
	for _, intent := range []reflex.Intent{reflex.RememberNote, reflex.RunRoutine, reflex.SettingFlip, "made_up"} {
		s := &planOf{intent: intent}
		r, tn := newRouter(t, Config{Name: "Oni", Reflex: s.slot})
		for _, u := range []Utterance{
			{Text: "remember that the exam is friday", Source: Wake},
			{Text: "Oni, remember that the exam is friday", Source: Open},
		} {
			u.Turn, u.At = tn.Barge(), t0
			d := r.Route(u)
			if d.Kind == Reflexed {
				t.Fatalf("%s from %v reached the daemon: %+v", intent, u.Source, d)
			}
			if d.Kind != NeedsPress || d.Line != ReflexNeedsPressLine || !Fits(d.Line) {
				t.Fatalf("%s from %v = %+v", intent, u.Source, d)
			}
		}
		for _, p := range s.press {
			if p.Valid() {
				t.Fatalf("wake or open mic handed the slot a valid press")
			}
		}
	}
	// an intent anyone may start goes through from wake
	s := &planOf{intent: reflex.VolumeUp}
	r, tn := newRouter(t, Config{Name: "Oni", Reflex: s.slot})
	if d := r.Route(Utterance{Text: "turn it up", Source: Wake, Turn: tn.Barge(), At: t0}); d.Kind != Reflexed || d.Press.Valid() {
		t.Fatalf("wake volume = %+v", d)
	}
}

// Open-mic speech that does not call the character by name is never
// planned; called by name, the slot hears it without the name.
func TestOpenMicIsPlannedOnlyWhenAddressed(t *testing.T) {
	s := &planOf{intent: reflex.OpenApp}
	r, tn := newRouter(t, Config{Name: "Oni", Reflex: s.slot})
	for _, text := range []string{"open files", "open files please", "Sam, open files", "Oni", "hey Oni"} {
		if d := r.Route(Utterance{Text: text, Source: Open, Turn: tn.Barge(), At: t0}); d.Kind == Reflexed {
			t.Errorf("%q from open mic was planned: %+v", text, d)
		}
	}
	if len(s.heard) != 0 {
		t.Fatalf("the slot heard %q", s.heard)
	}
	for _, text := range []string{"Oni, open files", "hey oni open files", "open files, Oni"} {
		if d := r.Route(Utterance{Text: text, Source: Open, Turn: tn.Barge(), At: t0}); d.Kind != Reflexed {
			t.Errorf("%q = %+v", text, d)
		}
	}
	for _, h := range s.heard {
		if h != "open files" {
			t.Fatalf("the slot heard %q, want the name taken off", s.heard)
		}
	}
	// with no name, open mic is never addressed
	nameless, tn2 := newRouter(t, Config{Reflex: s.slot})
	if d := nameless.Route(Utterance{Text: "open files", Source: Open, Turn: tn2.Barge(), At: t0}); d.Kind == Reflexed {
		t.Fatalf("a nameless character's open mic was planned: %+v", d)
	}
}

// A slot that does not take the utterance (no rule matched and the
// decision route is down) leaves it on the slow path, never dropped.
func TestReflexMissGoesToTheSlowPath(t *testing.T) {
	r, tn := newRouter(t, Config{Name: "Oni", Reflex: func(Utterance, Press) (Decision, bool) { return Decision{}, false }})
	for _, src := range []Source{Chord, Wake} {
		id := tn.Barge()
		d := r.Route(Utterance{Text: "could you fire up the notepad thing", Source: src, Turn: id, At: t0})
		if d.Kind != Asked || d.Ask == nil || d.Ask.Question != "could you fire up the notepad thing" {
			t.Fatalf("%v: %+v", src, d)
		}
	}
}
