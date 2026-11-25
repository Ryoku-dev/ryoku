package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAccuseOf_FixAccusedForExpressing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "he's been accused for expressing personal opinions, on his personal blog.", NewAccuseOf(), "he's been accused of expressing personal opinions, on his personal blog.")
}

func TestAccuseOf_FixAccusedForAtEnd(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That's not what they were accused for.", NewAccuseOf(), "That's not what they were accused of.")
}

func TestAccuseOf_FixAccusedThemFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "nobody accused them for being rational", NewAccuseOf(), "nobody accused them of being rational")
}

func TestAccuseOf_FixAccusedYouOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have never accused you for placing virus.", NewAccuseOf(), "I have never accused you of placing virus.")
}

func TestAccuseOf_FixGotAccusedFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So anti-phishing folks got accused for phishing.", NewAccuseOf(), "So anti-phishing folks got accused of phishing.")
}

func TestAccuseOf_FixAccuseForBeing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Emacs is not the SW application I would accuse for being bloated, quite the opposite in fact.", NewAccuseOf(), "Emacs is not the SW application I would accuse of being bloated, quite the opposite in fact.")
}

func TestAccuseOf_FixAccuseHimFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Lots of people accuse him for it and compare it to running things on WINE.", NewAccuseOf(), "Lots of people accuse him of it and compare it to running things on WINE.")
}

func TestAccuseOf_FixAccuseYouFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "do you mean that someone accuse you for distributing virus in your open-sourced code", NewAccuseOf(), "do you mean that someone accuse you of distributing virus in your open-sourced code")
}

func TestAccuseOf_FixAccusingMeFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Once I dreamed about receiving an anonymous hate mail accusing me for wasting other developer's precious time.", NewAccuseOf(), "Once I dreamed about receiving an anonymous hate mail accusing me of wasting other developer's precious time.")
}

func TestAccuseOf_FixAccusedAs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The website collected controversy as those studies were accused as not being very impartial and potentially misleading.", NewAccuseOf(), "The website collected controversy as those studies were accused of not being very impartial and potentially misleading.")
}

func TestAccuseOf_DontFlagTheAccused(t *testing.T) {
	linttest.AssertNoLints(t, "You are a defense lawyer defending the accused for a murder case.", NewAccuseOf())
}

func TestAccuseOf_DontFlagForTheNthTime(t *testing.T) {
	linttest.AssertNoLints(t, "I was accused for the first time of my writing looking AI-generated.", NewAccuseOf())
}
