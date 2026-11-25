package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestGoSoFarAsTo_GoSoFarTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'd even go so far to say as it's a good way to get started with getting things onto ...", NewGoSoFarAsTo(), "I'd even go so far as to say as it's a good way to get started with getting things onto ...")
}

func TestGoSoFarAsTo_GoesSoFarTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I believe Java goes so far to even throw a runtime exception.", NewGoSoFarAsTo(), "I believe Java goes so far as to even throw a runtime exception.")
}

func TestGoSoFarAsTo_GoneSoFarTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've gone so far to reinstall Mac OS, which got the runner to finally start", NewGoSoFarAsTo(), "I've gone so far as to reinstall Mac OS, which got the runner to finally start")
}

func TestGoSoFarAsTo_WentSoFarTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've read these posts but only went so far to conclude that I need to potentially add sql statements into the blocks", NewGoSoFarAsTo(), "I've read these posts but only went so far as to conclude that I need to potentially add sql statements into the blocks")
}

func TestGoSoFarAsTo_WentSoFarToAdverb(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I even went so far to manually replace the .AppImage with a different file in the Applications folder", NewGoSoFarAsTo(), "I even went so far as to manually replace the .AppImage with a different file in the Applications folder")
}

func TestGoSoFarAsTo_DontFlagGoingSoFarTo(t *testing.T) {
	t.Skip("A false positive we can't detect due to the next word being a verb lemma")
	linttest.AssertNoLints(t, "Why dictate that the system must be canonically described through a textual syntax – especially after going so far to make that unnecessary?", NewGoSoFarAsTo())
}

func TestGoSoFarAsTo_DontFlagGoesSoFarTo(t *testing.T) {
	linttest.AssertNoLints(t, "... even so much that one line goes so far to the right", NewGoSoFarAsTo())
}

func TestGoSoFarAsTo_DontFlagGoSoFarTo(t *testing.T) {
	linttest.AssertNoLints(t, "Unfortunetly, our logs don't go so far to that time, but I found something interesting.", NewGoSoFarAsTo())
}
