package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAllIntentsAndPurposes_FixForIntended(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The details tag should be treated like a div for all intended purposes, but unsure where in the selection logic", NewAllIntentsAndPurposes(), "The details tag should be treated like a div for all intents and purposes, but unsure where in the selection logic")
}

func TestAllIntentsAndPurposes_FixForIntense(t *testing.T) {
	linttest.AssertSuggestionResult(t, "For all intense purposes, I thought your code was really well written", NewAllIntentsAndPurposes(), "For all intents and purposes, I thought your code was really well written")
}

func TestAllIntentsAndPurposes_FixForIntensive(t *testing.T) {
	linttest.AssertSuggestionResult(t, "MultiNode could be for all intensive purposes, the same as Node sans the way it creates the command line arguments for the process.", NewAllIntentsAndPurposes(), "MultiNode could be for all intents and purposes, the same as Node sans the way it creates the command line arguments for the process.")
}

func TestAllIntentsAndPurposes_FixForIntrinsicPurposes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "For all intrinsic purposes I think you are wrong.", NewAllIntentsAndPurposes(), "For all intents and purposes I think you are wrong.")
}

func TestAllIntentsAndPurposes_FixInIntensePurposes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the solution has some rules than in all intense purposes is not necessarily database driven", NewAllIntentsAndPurposes(), "the solution has some rules than for all intents and purposes is not necessarily database driven")
}

func TestAllIntentsAndPurposes_FixToIntensivePurposes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "To all intensive purposes, for the consumer, a view is a table", NewAllIntentsAndPurposes(), "To all intents and purposes, for the consumer, a view is a table")
}

func TestAllIntentsAndPurposes_FixAtIntentsAnd(t *testing.T) {
	linttest.AssertSuggestionResult(t, "can be thought of, at all intents and purposes, as a controlled cache", NewAllIntentsAndPurposes(), "can be thought of, for all intents and purposes, as a controlled cache")
}

func TestAllIntentsAndPurposes_FixByIntentsAnd(t *testing.T) {
	linttest.AssertSuggestionResult(t, "so by all intents and purposes one is not nested into another", NewAllIntentsAndPurposes(), "so for all intents and purposes one is not nested into another")
}

func TestAllIntentsAndPurposes_FixForExtentsAnd(t *testing.T) {
	linttest.AssertSuggestionResult(t, "#include, for all extents and purposes (if you take the preprocessor out) just copies the file", NewAllIntentsAndPurposes(), "#include, for all intents and purposes (if you take the preprocessor out) just copies the file")
}

func TestAllIntentsAndPurposes_DontFlagForIntentsAnd(t *testing.T) {
	linttest.AssertNoLints(t, "with the previous previous setting still present and for all intents and purposes seems enabled", NewAllIntentsAndPurposes())
}

func TestAllIntentsAndPurposes_FixFromIntentsAnd(t *testing.T) {
	linttest.AssertSuggestionResult(t, "act as a full archive node from all intents and purposes", NewAllIntentsAndPurposes(), "act as a full archive node for all intents and purposes")
}

func TestAllIntentsAndPurposes_FixInIntentsAnd(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I posted #20493 asking about, in all intents and purposes, deno info", NewAllIntentsAndPurposes(), "I posted #20493 asking about, for all intents and purposes, deno info")
}

func TestAllIntentsAndPurposes_FixOnIntentsAnd(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It depends on all intents and purposes what you want to do.", NewAllIntentsAndPurposes(), "It depends for all intents and purposes what you want to do.")
}

func TestAllIntentsAndPurposes_FixThroughIntentsAnd(t *testing.T) {
	linttest.AssertSuggestionResult(t, "While, I know through all intents and purposes it is an ugly url to look at", NewAllIntentsAndPurposes(), "While, I know for all intents and purposes it is an ugly url to look at")
}

func TestAllIntentsAndPurposes_FixToExtentsAnd(t *testing.T) {
	linttest.AssertSuggestionResult(t, "and they were trying to find out how that would also affect the the personnel to all extents and purposes.", NewAllIntentsAndPurposes(), "and they were trying to find out how that would also affect the the personnel to all intents and purposes.")
}

func TestAllIntentsAndPurposes_DontFlagToIntentsAnd(t *testing.T) {
	linttest.AssertNoLints(t, "and they were trying to find out how that would also affect the the personnel to all intents and purposes.", NewAllIntentsAndPurposes())
}

func TestAllIntentsAndPurposes_FixWithIntentsAnd(t *testing.T) {
	linttest.AssertSuggestionResult(t, "With all intents and purposes the array should be As String since all values I'll be dealing with will be strings.", NewAllIntentsAndPurposes(), "For all intents and purposes the array should be As String since all values I'll be dealing with will be strings.")
}

func TestAllIntentsAndPurposes_FixByIntensivePurposes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "By all intensive purposes this should be working", NewAllIntentsAndPurposes(), "For all intents and purposes this should be working")
}

func TestAllIntentsAndPurposes_FixForIntenseAnd(t *testing.T) {
	linttest.AssertSuggestionResult(t, "to test my site, which for all intense and purposes works", NewAllIntentsAndPurposes(), "to test my site, which for all intents and purposes works")
}

func TestAllIntentsAndPurposes_FixInIntensivePurposes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "it should in all intensive purposes keep running", NewAllIntentsAndPurposes(), "it should for all intents and purposes keep running")
}

func TestAllIntentsAndPurposes_FixToIntenseAnd(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The other type, is to all intense and purposes a submit button to the browser", NewAllIntentsAndPurposes(), "The other type, is to all intents and purposes a submit button to the browser")
}

func TestAllIntentsAndPurposes_DontFlagForBasicallyAllIntentsAndPurposes(t *testing.T) {
	linttest.AssertNoLints(t, "For basically all intents and purposes, this works fine.", NewAllIntentsAndPurposes())
}

func TestAllIntentsAndPurposes_DontFlagForNearlyAllIntentsAndPurposes(t *testing.T) {
	linttest.AssertNoLints(t, "but for nearly all intents and purposes this should be negligable", NewAllIntentsAndPurposes())
}

func TestAllIntentsAndPurposes_DontFlagForPrettyMuchAllIntentsAndPurposes(t *testing.T) {
	linttest.AssertNoLints(t, "or for pretty much all intents and purposes, between Android devices", NewAllIntentsAndPurposes())
}

func TestAllIntentsAndPurposes_FalsePositiveFor99PercentOfAllIntentsAndPurposes(t *testing.T) {
	t.Skip("Rare and unusual false positive")
	linttest.AssertNoLints(t, "But for 99% of all intents and purposes they can be treated as lists.", NewAllIntentsAndPurposes())
}

func TestAllIntentsAndPurposes_FalsePositiveForUsConstitutionSpace(t *testing.T) {
	linttest.AssertNoLints(t, "Amendments, which, in either Case, shall be valid to all Intents and Purposes, as Part of this Constitution", NewAllIntentsAndPurposes())
}

func TestAllIntentsAndPurposes_FalsePositiveForUsConstitutionLineBreak(t *testing.T) {
	t.Skip("The linefeed should be treated as a space!")
	linttest.AssertNoLints(t, "Amendments, which, in either Case, shall be valid to all Intents and\nPurposes, as Part of this Constitution", NewAllIntentsAndPurposes())
}

func TestAllIntentsAndPurposes_FixAllIntentsAndPurpose(t *testing.T) {
	linttest.AssertSuggestionResult(t, "To all intents and purpose, the SEC no longer exists", NewAllIntentsAndPurposes(), "To all intents and purposes, the SEC no longer exists")
}

func TestAllIntentsAndPurposes_DontFlagConstitution(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Amendments, which, in either Case, shall be valid to all Intents and Purposes, as Part of this Constitution", NewAllIntentsAndPurposes(), "Amendments, which, in either Case, shall be valid to all Intents and Purposes, as Part of this Constitution")
}
