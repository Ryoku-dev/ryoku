package route

import (
	"testing"
)

// While Hands waits on a card, a yes or no from every source, the talk
// chord and toggle included, answers nothing: the router says to use the
// button, never calls the consent slot, and leaves Hands running.
func TestHandsCardRefusesEverySpokenAnswer(t *testing.T) {
	stops := 0
	r, tn := newRouter(t, Config{
		Name:      "Oni",
		Consent:   failingConsent{t},
		HandsCard: func() bool { return true },
		HandsStop: func() { stops++ },
	})
	id := tn.Barge()
	for _, s := range []Source{Chord, Toggle, Wake, Open} {
		for _, text := range []string{"yes", "Yes, do it.", "go ahead", "no", "Oni, yes", "approve it"} {
			d := r.Route(Utterance{Text: text, Source: s, Turn: id, At: t0})
			if d.Kind != NeedsPress || d.Line != FirmNeedsButtonLine {
				t.Fatalf("%s %q = %+v", s, text, d)
			}
		}
	}
	if stops != 0 {
		t.Fatalf("a spoken answer stopped Hands %d times", stops)
	}
	if !Fits(FirmNeedsButtonLine) {
		t.Fatalf("%q breaks the voice discipline", FirmNeedsButtonLine)
	}
}

// Every dismissal stops Hands from any source; a pressed utterance the
// Hands slot does not take stops it too, while wake and open mic speech
// that is no dismissal leaves it alone. The slot is reached only from a
// press.
func TestHandsStopsOnDismissalAndOnUnclaimedPress(t *testing.T) {
	stops := 0
	var handed []Source
	claim := false
	r, tn := newRouter(t, Config{
		Name:      "Oni",
		HandsStop: func() { stops++ },
		Hands: func(p Press, text string) bool {
			if !p.Valid() {
				t.Fatalf("hands given an invalid press for %q", text)
			}
			handed = append(handed, p.Source())
			return claim
		},
	})
	id := tn.Barge()
	route := func(text string, s Source) Decision {
		return r.Route(Utterance{Text: text, Source: s, Turn: id, At: t0})
	}
	for _, s := range []Source{Chord, Toggle, Wake, Open} {
		before := stops
		if d := route("stop", s); d.Kind != Dismissed || stops != before+1 {
			t.Fatalf("%s stop = %+v, stops %d", s, d, stops-before)
		}
	}
	before := stops
	for _, s := range []Source{Wake, Open} {
		route("fill the form on this page", s)
	}
	if stops != before || len(handed) != 0 {
		t.Fatalf("wake or open mic reached Hands: stops %d handed %v", stops-before, handed)
	}
	if d := route("fill the form on this page", Chord); d.Kind != Asked || stops != before+1 || len(handed) != 1 {
		t.Fatalf("an unclaimed press = %+v, stops %d", d, stops-before)
	}
	claim = true
	if d := route("fill the form on this page", Toggle); d.Kind != HandsTook || stops != before+1 || len(handed) != 2 {
		t.Fatalf("a claimed press = %+v, stops %d", d, stops-before)
	}
}
