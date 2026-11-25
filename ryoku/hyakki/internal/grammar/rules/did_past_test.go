package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestDidPast_EdDidForked(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Did they forked the repo?", NewDidPast(dict.Curated()), "Did they fork the repo?")
}

func TestDidPast_DDidUsed(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It didn't used a macro.", NewDidPast(dict.Curated()), "It didn't use a macro.")
}

func TestDidPast_YDidFried(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I hope that didn't fried any chips!", NewDidPast(dict.Curated()), "I hope that didn't fry any chips!")
}

func TestDidPast_DoubedConsonantLogged(t *testing.T) {
	linttest.AssertSuggestionResult(t, "There was a segfault but it did logged the error.", NewDidPast(dict.Curated()), "There was a segfault but it did log the error.")
}

func TestDidPast_DidPast(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Did went", NewDidPast(dict.Curated()), "Did go")
}

func TestDidPast_DidPastWithApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Didn't saw", NewDidPast(dict.Curated()), "Didn't see")
}

func TestDidPast_DidntPastNoApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Didnt had", NewDidPast(dict.Curated()), "Didnt have")
}

func TestDidPast_DidIHeard(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Did I heard", NewDidPast(dict.Curated()), "Did I hear")
}

func TestDidPast_DidIHeardWithApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Didn't we heard", NewDidPast(dict.Curated()), "Didn't we hear")
}

func TestDidPast_DidntIForgotNoApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Didnt he forgot", NewDidPast(dict.Curated()), "Didnt he forget")
}

func TestDidPast_IgnoreLemmaSameAsPastTense(t *testing.T) {
	linttest.AssertNoLints(t, "Did read", NewDidPast(dict.Curated()))
}

func TestDidPast_FixDidYouCmae(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How did you came to this", NewDidPast(dict.Curated()), "How did you come to this")
}

func TestDidPast_FixDidYouWrote(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm very interested in the script, if you did wrote it.", NewDidPast(dict.Curated()), "I'm very interested in the script, if you did write it.")
}

func TestDidPast_FixDidntHad(t *testing.T) {
	linttest.AssertSuggestionResult(t, "and i DO know that i didnt had any Terracota", NewDidPast(dict.Curated()), "and i DO know that i didnt have any Terracota")
}

func TestDidPast_DidYouWent(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Did you went out of memory maybe?", NewDidPast(dict.Curated()), "Did you go out of memory maybe?")
}

func TestDidPast_FixDidNeeded(t *testing.T) {
	linttest.AssertSuggestionResult(t, "since our CI was broken this did needed to be done", NewDidPast(dict.Curated()), "since our CI was broken this did need to be done")
}

func TestDidPast_FixDidThought(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I did thought of adding it as a tooltip on hover", NewDidPast(dict.Curated()), "I did think of adding it as a tooltip on hover")
}

func TestDidPast_FixDidWanted(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I did wanted catch all errors in my previous example.", NewDidPast(dict.Curated()), "I did want catch all errors in my previous example.")
}

func TestDidPast_FixDidNotChanged(t *testing.T) {
	linttest.AssertSuggestionResult(t, "freeing space and reboot frequently did not changed anything", NewDidPast(dict.Curated()), "freeing space and reboot frequently did not change anything")
}

func TestDidPast_IgnoreDidYouRead(t *testing.T) {
	linttest.AssertNoLints(t, "Did You Read the Instructions?", NewDidPast(dict.Curated()))
}

func TestDidPast_Issue3916DidntUnderstood(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I didn't understood the problem.", NewDidPast(dict.Curated()), "I didn't understand the problem.")
}

func TestDidPast_Issue3916DidUnderstood(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I did understood the problem.", NewDidPast(dict.Curated()), "I did understand the problem.")
}

func TestDidPast_Issue3916CorrectUsage(t *testing.T) {
	linttest.AssertNoLints(t, "I didn't understand the problem.", NewDidPast(dict.Curated()))
}
