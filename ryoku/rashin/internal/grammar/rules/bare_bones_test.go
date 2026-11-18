package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestBareBones_SingularAttributiveSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "bare bone csv-parse won't parse anything", NewBareBones(), "bare bones csv-parse won't parse anything")
}

func TestBareBones_SingularAttributiveHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is a simple bare-bone module which allows background playing from a HTTP stream, using ExoPlayer.", NewBareBones(), "This is a simple bare-bones module which allows background playing from a HTTP stream, using ExoPlayer.")
}

func TestBareBones_DontFlagPluralAttributiveHyphen(t *testing.T) {
	linttest.AssertNoLints(t, "For the bare-bones version, just display all tags, expanded.", NewBareBones())
}

func TestBareBones_DontFlagPluralAttributiveSpace(t *testing.T) {
	linttest.AssertNoLints(t, "Support for when JS disabled + a text-only bare bones version", NewBareBones())
}

func TestBareBones_DontFlagPluralPredicativeSpace(t *testing.T) {
	linttest.AssertNoLints(t, "Even though they are bare bones, they still require some design knowledge to be very successful.", NewBareBones())
}

func TestBareBones_DontFlagPluralPredicativeHyphen(t *testing.T) {
	linttest.AssertNoLints(t, "The core is bare-bones, there are numerous standard protocols since, and many other are in standardization.", NewBareBones())
}

func TestBareBones_SingularPredicativeSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It is bare bone because all my views are dinamically created", NewBareBones(), "It is bare bones because all my views are dinamically created")
}

func TestBareBones_SingularPredicativeHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This repo has been created, but it is bare-bone.", NewBareBones(), "This repo has been created, but it is bare-bone.")
}
