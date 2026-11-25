package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestNakedEye_DontFlagHidingAdjNounFrom(t *testing.T) {
	t.Skip("Underscores are not parts of words")
	linttest.AssertNoLints(t, "Once the opt-in is complete, the obtained lock() and unlock() utilities enable hiding __experimental APIs from the naked eye:", NewNakedEye())
}

func TestNakedEye_DontFlagEasilyHideNounFrom(t *testing.T) {
	linttest.AssertNoLints(t, "But there is something we forgot about, something that allows one to easily hide things from the naked eye ... macros!", NewNakedEye())
}

func TestNakedEye_DontFlagWellHiddenFrom(t *testing.T) {
	linttest.AssertNoLints(t, "We successfully tampered with the release, well hidden from the naked eye.", NewNakedEye())
}

func TestNakedEye_ButFromTheNakedEye(t *testing.T) {
	linttest.AssertSuggestionResult(t, "As you can see from the pic the pedestrian is being detected but from the naked eye it can't be seen due to the slope.", NewNakedEye(), "As you can see from the pic the pedestrian is being detected but to the naked eye it can't be seen due to the slope.")
}

func TestNakedEye_EvenFromTheNakedEye(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the Photoshop one when even from the naked eye you can tell that if you were to zoom into", NewNakedEye(), "the Photoshop one when even with the naked eye you can tell that if you were to zoom into")
}

func TestNakedEye_LooksLegitToTheNakedEye(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In fact, it looks like legit 1080p from the naked eye I guess... 😭", NewNakedEye(), "In fact, it looks like legit 1080p to the naked eye I guess... 😭")
}

func TestNakedEye_ObservedFromTheNakedEye(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... when observed from the naked eye for the bone micro-architecture of the osteoporotic and healthy cases ...", NewNakedEye(), "... when observed with the naked eye for the bone micro-architecture of the osteoporotic and healthy cases ...")
}

func TestNakedEye_BeingPrivateHiddenFrom(t *testing.T) {
	linttest.AssertNoLints(t, "... which makes the entire concept of it being private hidden from the naked eye.", NewNakedEye())
}

func TestNakedEye_DontFlagKeepDetNounFrom(t *testing.T) {
	linttest.AssertNoLints(t, "... but how can I keep this key from the naked eyes?", NewNakedEye())
}

func TestNakedEye_JustFromTheNakedEye(t *testing.T) {
	linttest.AssertSuggestionResult(t, "What I want is, just from the naked eye, I think there are only 5 colors on this picture", NewNakedEye(), "What I want is, just with the naked eye, I think there are only 5 colors on this picture")
}

func TestNakedEye_InvisibilityFromTheNakedEye(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Considering the real security implications of true invisibility from the naked eye", NewNakedEye(), "Considering the real security implications of true invisibility to the naked eye")
}
