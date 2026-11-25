package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestQuiteQuiet_FixQuietAdverb(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Rendering videos 145 frames, with lightx loras for 2.1 i experience reboots quiet often.", NewQuiteQuiet(), "Rendering videos 145 frames, with lightx loras for 2.1 i experience reboots quite often.")
}

func TestQuiteQuiet_FixQuietAdjective(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... has been already reported multiple times and I find it quiet dumb that it still exists", NewQuiteQuiet(), "... has been already reported multiple times and I find it quite dumb that it still exists")
}

func TestQuiteQuiet_FixVeryQuite(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's very quite here at night.", NewQuiteQuiet(), "It's very quiet here at night.")
}

func TestQuiteQuiet_FixDoesntQuiet(t *testing.T) {
	linttest.AssertSuggestionResult(t, "doesn't quiet", NewQuiteQuiet(), "doesn't quite")
}

func TestQuiteQuiet_FixDoesntQuietTypographicalApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "doesn’t quiet", NewQuiteQuiet(), "doesn’t quite")
}

func TestQuiteQuiet_FixDoesntQuietInContext(t *testing.T) {
	linttest.AssertSuggestionResult(t, "When we got the car back into the workshop, we actually managed to get it running and driving, but it doesn't quiet run right, and doesn't really let me rev it.", NewQuiteQuiet(), "When we got the car back into the workshop, we actually managed to get it running and driving, but it doesn't quite run right, and doesn't really let me rev it.")
}

func TestQuiteQuiet_DontFlagQuietLight(t *testing.T) {
	linttest.AssertLintCount(t, "The quiet lights in the houses", NewQuiteQuiet(), 0)
}

func TestQuiteQuiet_DontFlagQuietTill(t *testing.T) {
	linttest.AssertLintCount(t, "You’d better try and sit quiet till morning.", NewQuiteQuiet(), 0)
}

func TestQuiteQuiet_FixCantQuiet(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I can't quiet read it", NewQuiteQuiet(), "I can't quite read it")
}

func TestQuiteQuiet_FixWontQuiet(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It won't quiet fit", NewQuiteQuiet(), "It won't quite fit")
}

func TestQuiteQuiet_FixCouldntQuiet(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I couldn't quiet understand everything", NewQuiteQuiet(), "I couldn't quite understand everything")
}

func TestQuiteQuiet_FixButItsNotQuiteClear1956(t *testing.T) {
	linttest.AssertNoLints(t, "But it's not quite clear", NewQuiteQuiet())
}

func TestQuiteQuiet_DontFlagAdvQuite1971(t *testing.T) {
	linttest.AssertNoLints(t, "It’s actually quite smart. It’s really quite smart. The proof is actually quite neat. Actually really quite simple. It’s actually quite strong. The Sneetches got really quite smart on that day.", NewQuiteQuiet())
}

func TestQuiteQuiet_Issue2003(t *testing.T) {
	linttest.AssertNoLints(t, "The namespaces are generally quite short", NewQuiteQuiet())
}

func TestQuiteQuiet_DontFlagGoQuietFor(t *testing.T) {
	linttest.AssertNoLints(t, "If I go quiet for a week… yeah I'm dead.", NewQuiteQuiet())
}

func TestQuiteQuiet_DontFlagWentQuietAbout(t *testing.T) {
	linttest.AssertNoLints(t, "He went quiet about the whole thing.", NewQuiteQuiet())
}

func TestQuiteQuiet_DontFlagKeepQuietAbout(t *testing.T) {
	linttest.AssertNoLints(t, "She kept quiet about what happened.", NewQuiteQuiet())
}

func TestQuiteQuiet_DontFlagStayQuietDuring(t *testing.T) {
	linttest.AssertNoLints(t, "Please stay quiet during the presentation.", NewQuiteQuiet())
}

func TestQuiteQuiet_DontFlagFellQuietAfter(t *testing.T) {
	linttest.AssertNoLints(t, "The room fell quiet after the announcement.", NewQuiteQuiet())
}

func TestQuiteQuiet_DontFlagRemainedQuietIn(t *testing.T) {
	linttest.AssertNoLints(t, "She remained quiet in meetings.", NewQuiteQuiet())
}

func TestQuiteQuiet_DontFlagGrewQuietOn(t *testing.T) {
	linttest.AssertNoLints(t, "He grew quiet on the matter.", NewQuiteQuiet())
}

func TestQuiteQuiet_DontFlagBeQuietFor(t *testing.T) {
	linttest.AssertNoLints(t, "Be quiet for a moment.", NewQuiteQuiet())
}

func TestQuiteQuiet_DontFlagQuietAnd(t *testing.T) {
	linttest.AssertNoLints(t, "Stay quiet and listen.", NewQuiteQuiet())
}

func TestQuiteQuiet_DontFlagQuietBut(t *testing.T) {
	linttest.AssertNoLints(t, "She was quiet but firm.", NewQuiteQuiet())
}

func TestQuiteQuiet_DontFlagQuietNow(t *testing.T) {
	linttest.AssertNoLints(t, "Be quiet now.", NewQuiteQuiet())
}

func TestQuiteQuiet_DontFlagQuietThere(t *testing.T) {
	linttest.AssertNoLints(t, "It's quiet there.", NewQuiteQuiet())
}

func TestQuiteQuiet_DontFlagQuietDown(t *testing.T) {
	linttest.AssertNoLints(t, "Quiet down, everyone.", NewQuiteQuiet())
}

func TestQuiteQuiet_DontFlagQuietEnough(t *testing.T) {
	linttest.AssertNoLints(t, "The room was quiet enough.", NewQuiteQuiet())
}

func TestQuiteQuiet_DontFlagQuietLately(t *testing.T) {
	linttest.AssertNoLints(t, "It's been quiet lately.", NewQuiteQuiet())
}

func TestQuiteQuiet_StillCatchesQuietRemarkable(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That was quiet remarkable.", NewQuiteQuiet(), "That was quite remarkable.")
}

func TestQuiteQuiet_StillCatchesQuietImpressive(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That was quiet impressive.", NewQuiteQuiet(), "That was quite impressive.")
}

func TestQuiteQuiet_DontFlagProbablyQuiteDoable3560(t *testing.T) {
	linttest.AssertNoLints(t, "It's probably quite doable.", NewQuiteQuiet())
}

func TestQuiteQuiet_DontFlagProbablyQuiteReasonable3560(t *testing.T) {
	linttest.AssertNoLints(t, "That seems probably quite reasonable.", NewQuiteQuiet())
}

func TestQuiteQuiet_DontFlagQuiteHappy3560(t *testing.T) {
	linttest.AssertNoLints(t, "I'm quite happy with the result.", NewQuiteQuiet())
}

func TestQuiteQuiet_DontFlagQuiteLarge3560(t *testing.T) {
	linttest.AssertNoLints(t, "The project is quite large.", NewQuiteQuiet())
}
