package route

import (
	"testing"

	"ryoku-rashin/internal/oni/note"
	"ryoku-rashin/internal/oni/recall"
	"ryoku-rashin/internal/oni/reflex"
)

func on() bool  { return true }
func off() bool { return false }

// Memory commands and recall questions come before the Reflex slot, so a
// forget never becomes a quick command, and a note stays the slot's. A
// bare "forget that" or "forget it" stays a dismissal.
func TestMemoryAndRecallComeBeforeTheReflexSlot(t *testing.T) {
	s := &planOf{intent: reflex.RememberNote}
	r, tn := newRouter(t, Config{Name: "Oni", Reflex: s.slot, Memory: on, Recall: on})
	cases := []struct {
		say  string
		want Kind
	}{
		{"forget that", Dismissed},
		{"forget that I live in Lisbon", Noted},
		{"forget what I just said", Noted},
		{"no, I meant Porto", Noted},
		{"correct tea to coffee", Noted},
		{"what do you know about me", Noted},
		{"what did I ask Claude about eigenvalues", Recalled},
		{"what's in my vault about rust", Recalled},
		{"don't forget that the bins go out", Reflexed},
		{"remember that I live in Lisbon", Reflexed},
	}
	for _, c := range cases {
		id := tn.Barge()
		d := r.Route(Utterance{Text: c.say, Source: Chord, Turn: id, At: t0})
		if d.Kind != c.want {
			t.Errorf("%q = %v, want %v", c.say, d.Kind, c.want)
		}
		switch d.Kind {
		case Noted:
			if d.Note == nil || !d.Press.Valid() {
				t.Errorf("%q: %+v", c.say, d)
			}
		case Recalled:
			if d.Recall == nil || d.Recall.Topic == "" {
				t.Errorf("%q: %+v", c.say, d)
			}
		}
	}
	id := tn.Barge()
	d := r.Route(Utterance{Text: "Oni, forget that I live in Lisbon", Source: Toggle, Turn: id, At: t0})
	if d.Kind != Noted || *d.Note != (note.Command{Kind: note.Forget, Arg: "I live in Lisbon"}) {
		t.Fatalf("addressed: %+v", d)
	}
}

// A forget or a correction needs a Press; what Oni knows and a recall do
// not. Open mic reaches neither unless it calls the character.
func TestWritesNeedAPressAndOpenMicNeedsTheName(t *testing.T) {
	r, tn := newRouter(t, Config{Name: "Oni", Memory: on, Recall: on})
	for _, src := range []Source{Wake, Open} {
		for _, say := range []string{"Oni forget what I just said", "Oni no, I meant Porto", "Oni correct tea to coffee"} {
			id := tn.Barge()
			if d := r.Route(Utterance{Text: say, Source: src, Turn: id, At: t0}); d.Kind != NeedsPress || d.Line != ReflexNeedsPressLine || d.Note != nil {
				t.Errorf("%v %q = %+v", src, say, d)
			}
		}
		id := tn.Barge()
		if d := r.Route(Utterance{Text: "Oni what do you know about me", Source: src, Turn: id, At: t0}); d.Kind != Noted || d.Press.Valid() {
			t.Errorf("%v know = %+v", src, d)
		}
		id = tn.Barge()
		if d := r.Route(Utterance{Text: "Oni what did I ask about sourdough", Source: src, Turn: id, At: t0}); d.Kind != Recalled || *d.Recall != (recall.Query{Where: recall.Chats, Topic: "sourdough"}) {
			t.Errorf("%v recall = %+v", src, d)
		}
	}
	id := tn.Barge()
	if d := r.Route(Utterance{Text: "forget what I just said", Source: Open, Turn: id, At: t0}); d.Kind == Noted || d.Kind == NeedsPress {
		t.Fatalf("unaddressed open mic reached the memory slot: %+v", d)
	}
}

// With their switches off the phrases are ordinary questions.
func TestOffIsAnOrdinaryAsk(t *testing.T) {
	for _, c := range []Config{{Name: "Oni"}, {Name: "Oni", Memory: off, Recall: off}} {
		r, tn := newRouter(t, c)
		for _, say := range []string{"forget what I just said", "what do you know about me", "what did I ask Claude about eigenvalues"} {
			id := tn.Barge()
			if d := r.Route(Utterance{Text: say, Source: Chord, Turn: id, At: t0}); d.Kind != Asked || d.Note != nil || d.Recall != nil {
				t.Errorf("%q = %+v", say, d)
			}
		}
	}
}
