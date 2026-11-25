package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestFeelFell_FixIFellLike(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But I fell like i am having a knot in my brain ...", NewFeelFell(), "But I feel like i am having a knot in my brain ...")
}

func TestFeelFell_FixIfYouFellLikeIt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If you fell like it create w2ui-postgres for server side implementation", NewFeelFell(), "If you feel like it create w2ui-postgres for server side implementation")
}

func TestFeelFell_FixIDontFellLike(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But with this bug in place, I don't fell like asking the student to work with this tool", NewFeelFell(), "But with this bug in place, I don't feel like asking the student to work with this tool")
}

func TestFeelFell_FixFellComfortable(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Technology that I fell comfortable to wok Php,Laravel, Javascript,Vue, Jquery, MySqli, sqLite.", NewFeelFell(), "Technology that I feel comfortable to wok Php,Laravel, Javascript,Vue, Jquery, MySqli, sqLite.")
}

func TestFeelFell_FixFellGood(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've ha a touch of the flu and didn't fell good enough to mess with the computer.", NewFeelFell(), "I've ha a touch of the flu and didn't feel good enough to mess with the computer.")
}

func TestFeelFell_FixDidntFell(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They have served me well, and I didn't fell that it's a gamble.", NewFeelFell(), "They have served me well, and I didn't feel that it's a gamble.")
}

func TestFeelFell_FixFellFree(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Please fell free to add more songs.", NewFeelFell(), "Please feel free to add more songs.")
}

func TestFeelFell_FixFellRight(t *testing.T) {
	t.Skip("Needs more context or better heuristics")
	linttest.AssertSuggestionResult(t, "It may fell right first but only causes confusion in long run.", NewFeelFell(), "It may feel right first but only causes confusion in long run.")
}

func TestFeelFell_DontFlagFellRightInto(t *testing.T) {
	linttest.AssertNoLints(t, "I followed the instructions in the browser, and waited, then it fell right into shape, and the system is working out.", NewFeelFell())
}

func TestFeelFell_DontFlagFellRightThrough(t *testing.T) {
	linttest.AssertNoLints(t, "In this case the whole Piper context menu entry is missing since the uncaught exception fell right through the whole context menu factory.", NewFeelFell())
}

func TestFeelFell_FixDoesNotFellComfortable(t *testing.T) {
	linttest.AssertSuggestionResult(t, "she does not fell comfortable with the \" iso \"-format", NewFeelFell(), "she does not feel comfortable with the \" iso \"-format")
}

func TestFeelFell_DontFlagDidntFellForIt(t *testing.T) {
	t.Skip("Needs more context or better heuristics")
	linttest.AssertNoLints(t, "I even tried to trick someone else to delete and add the device but he didn't fell for it...", NewFeelFell())
}

func TestFeelFell_FixFellThat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I fell that a libSQL adapter would be a reasonable addition to the core offering.", NewFeelFell(), "I feel that a libSQL adapter would be a reasonable addition to the core offering.")
}

func TestFeelFell_FixFellIt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I personally fell it makes the screens difficult to use", NewFeelFell(), "I personally feel it makes the screens difficult to use")
}

func TestFeelFell_FixFellIts(t *testing.T) {
	linttest.AssertSuggestionResult(t, "but I fell it's too late to update that specific part of the API", NewFeelFell(), "but I feel it's too late to update that specific part of the API")
}

func TestFeelFell_FixFellIm(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I fell I'm missing sth and I need help.", NewFeelFell(), "I feel I'm missing sth and I need help.")
}

func TestFeelFell_FixFellWe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "i fell we will have to directly use BigDecimal here for Json encoding", NewFeelFell(), "i feel we will have to directly use BigDecimal here for Json encoding")
}

func TestFeelFell_FixFellI(t *testing.T) {
	linttest.AssertSuggestionResult(t, "feel free to reopen if you fell I have missed something", NewFeelFell(), "feel free to reopen if you feel I have missed something")
}

func TestFeelFell_FixFellYou(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But, I maybe fell you are stepping away from what a Markdown link actually is", NewFeelFell(), "But, I maybe feel you are stepping away from what a Markdown link actually is")
}
