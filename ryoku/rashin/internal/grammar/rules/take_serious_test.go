package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestTakeSerious_TakeIt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I take it serious.", NewTakeSerious(), "I take it seriously.")
}

func TestTakeSerious_TakeThis(t *testing.T) {
	t.Skip("'This' and 'that', which can be determiners and pronouns, are not handled properly by `NominalPhrase`")
	linttest.AssertSuggestionResult(t, "What's more important is, that it's impossible to actually take this serious when ...", NewTakeSerious(), "What's more important is, that it's impossible to actually take this seriously when ...")
}

func TestTakeSerious_NotTakeSecurity(t *testing.T) {
	linttest.AssertSuggestionResult(t, "When you say someone does not take security serious you are being judgemental / destructive.", NewTakeSerious(), "When you say someone does not take security seriously you are being judgemental / destructive.")
}

func TestTakeSerious_WeTakeSecurity(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We take security serious.", NewTakeSerious(), "We take security seriously.")
}

func TestTakeSerious_TakeMe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Yeah , don't take me serious , i do this as a hobby - jusspatel.", NewTakeSerious(), "Yeah , don't take me seriously , i do this as a hobby - jusspatel.")
}

func TestTakeSerious_TakenAdv(t *testing.T) {
	t.Skip("Passive voice and adverbs are not yet supported")
	linttest.AssertSuggestionResult(t, "This is not meant to be taken overly serious", NewTakeSerious(), "This is not meant to be taken overly seriously")
}

func TestTakeSerious_TakesTheseNumbers(t *testing.T) {
	linttest.AssertSuggestionResult(t, "if a program actually takes these numbers serious the results could be catastrophic.", NewTakeSerious(), "if a program actually takes these numbers seriously the results could be catastrophic.")
}

func TestTakeSerious_TakesBf(t *testing.T) {
	t.Skip("'No one' is not handled properly by `NominalPhrase`")
	linttest.AssertSuggestionResult(t, "And obviously no one takes brainfuck serious as a language.", NewTakeSerious(), "And obviously no one takes brainfuck seriously as a language.")
}

func TestTakeSerious_TakenVery(t *testing.T) {
	t.Skip("Adverbs are not yet supported")
	linttest.AssertSuggestionResult(t, "Hmm flaky soldering iron is something that must be taken very serious.", NewTakeSerious(), "Hmm flaky soldering iron is something that must be taken very seriously.")
}
