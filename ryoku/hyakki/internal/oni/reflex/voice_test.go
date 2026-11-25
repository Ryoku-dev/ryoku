package reflex_test

import (
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/reflex"
	"ryoku-rashin/internal/oni/voice/hygiene"
	"ryoku-rashin/internal/oni/voice/route"
)

// Every line Oni says keeps the voice discipline: one sentence of at most
// route.MaxLeadWords words.
func TestLinesKeepTheVoiceDiscipline(t *testing.T) {
	args := []string{"", reflex.MuteOn, reflex.MuteOff, reflex.MediaPlay, reflex.MediaPause, reflex.MediaToggle}
	for _, in := range reflex.Registry() {
		for _, arg := range args {
			if l := reflex.LineFor(reflex.Action{Intent: in.ID, Arg: arg}); !route.Fits(l) {
				t.Errorf("%s/%q: %q does not fit", in.ID, arg, l)
			}
		}
	}
	for _, l := range reflex.Fixed {
		if !route.Fits(l) {
			t.Errorf("%q does not fit", l)
		}
	}
}

// A transcript the decoder looped on, cleaned the way the voice loop cleans
// it, plans one launch.
func TestHygieneThenParseLaunchesOnce(t *testing.T) {
	looped := strings.Repeat("open firefox ", 43)
	for _, text := range []string{hygiene.Collapse(looped), hygiene.DropRepeatedSentences(strings.Repeat("Open firefox please. ", 43))} {
		p, ok := reflex.Parse(text, time.Time{}, time.UTC, reflex.TestCat)
		if !ok || len(p.Actions) != 1 || p.Actions[0].Intent != reflex.OpenApp {
			t.Errorf("%q: %+v %v", text, p, ok)
		}
	}
}
