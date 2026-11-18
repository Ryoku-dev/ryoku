package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestOughtToBe_FlagsYouOutToBeAbleToSee(t *testing.T) {
	linttest.AssertSuggestionResult(t, "you out to be able to see", NewOughtToBe(), "you ought to be able to see")
}

func TestOughtToBe_FlagsAsItOutToBe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "as it out to be", NewOughtToBe(), "as it ought to be")
}

func TestOughtToBe_FlagsThenItOutToBe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "then it out to be", NewOughtToBe(), "then it ought to be")
}

func TestOughtToBe_IgnoresTurnedOutToBe(t *testing.T) {
	linttest.AssertLintCount(t, "It turned out to be fine.", NewOughtToBe(), 0)
}

func TestOughtToBe_IgnoresTurnsOutToBe(t *testing.T) {
	linttest.AssertLintCount(t, "It turns out to be fine.", NewOughtToBe(), 0)
}

func TestOughtToBe_IgnoresMakeItOutToBe(t *testing.T) {
	linttest.AssertLintCount(t, "It's not as simple as they make it out to be.", NewOughtToBe(), 0)
}

func TestOughtToBe_IgnoresMakesItOutToBe(t *testing.T) {
	linttest.AssertLintCount(t, "I think this rule may not be as smart as its definition makes it out to be.", NewOughtToBe(), 0)
}

func TestOughtToBe_IgnoresWorkedOutToBe(t *testing.T) {
	linttest.AssertLintCount(t, "It worked out to be $5.", NewOughtToBe(), 0)
}

func TestOughtToBe_IgnoresFiguredItOutToBe(t *testing.T) {
	linttest.AssertLintCount(t, "I figured it out to be a memory issue.", NewOughtToBe(), 0)
}

func TestOughtToBe_IgnoresTryItOutToBe(t *testing.T) {
	linttest.AssertLintCount(t, "Try it out to be sure.", NewOughtToBe(), 0)
}

func TestOughtToBe_IgnoresSeparateItOutToBe(t *testing.T) {
	linttest.AssertLintCount(t, "I want to separate it out to be able to process it later.", NewOughtToBe(), 0)
}

func TestOughtToBe_IgnoresRotateItOutToBe(t *testing.T) {
	linttest.AssertLintCount(t, "We will rotate it out to be eventually deleted.", NewOughtToBe(), 0)
}

func TestOughtToBe_IgnoresFleshItOutToBe(t *testing.T) {
	linttest.AssertLintCount(t, "This needs some work to flesh it out to be usable.", NewOughtToBe(), 0)
}
