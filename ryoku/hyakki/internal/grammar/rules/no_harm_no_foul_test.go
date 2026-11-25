package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestNoHarmNoFoul_DontFlagStandardWithoutComma(t *testing.T) {
	linttest.AssertNoLints(t, "no harm no foul", NewNoHarmNoFoul())
}

func TestNoHarmNoFoul_DontFlagStandardWithComma(t *testing.T) {
	linttest.AssertNoLints(t, "no harm, no foul", NewNoHarmNoFoul())
}

func TestNoHarmNoFoul_FlagNoHarmNoFowl(t *testing.T) {
	linttest.AssertSuggestionResult(t, "no harm no fowl", NewNoHarmNoFoul(), "no harm no foul")
}

func TestNoHarmNoFoul_FlagNoHarmCommaNoFowl(t *testing.T) {
	linttest.AssertSuggestionResult(t, "no harm, no fowl", NewNoHarmNoFoul(), "no harm, no foul")
}

func TestNoHarmNoFoul_FlagNoHarmNorFoul(t *testing.T) {
	linttest.AssertSuggestionResult(t, "no harm nor foul", NewNoHarmNoFoul(), "no harm no foul")
}

func TestNoHarmNoFoul_FlagNoHarmCommaNorFoul(t *testing.T) {
	linttest.AssertSuggestionResult(t, "no harm, nor foul", NewNoHarmNoFoul(), "no harm, no foul")
}

func TestNoHarmNoFoul_FlagNoHarmNorFowl(t *testing.T) {
	linttest.AssertSuggestionResult(t, "no harm nor fowl", NewNoHarmNoFoul(), "no harm no foul")
}

func TestNoHarmNoFoul_FlagNoHarmCommaNorFowl(t *testing.T) {
	linttest.AssertSuggestionResult(t, "no harm, nor fowl", NewNoHarmNoFoul(), "no harm, no foul")
}

func TestNoHarmNoFoul_FlagNorHarmNoFoul(t *testing.T) {
	linttest.AssertSuggestionResult(t, "nor harm no foul", NewNoHarmNoFoul(), "no harm no foul")
}

func TestNoHarmNoFoul_FlagNorHarmCommaNoFoul(t *testing.T) {
	linttest.AssertSuggestionResult(t, "nor harm, no foul", NewNoHarmNoFoul(), "no harm, no foul")
}

func TestNoHarmNoFoul_FlagNorHarmNoFowl(t *testing.T) {
	linttest.AssertSuggestionResult(t, "nor harm no fowl", NewNoHarmNoFoul(), "no harm no foul")
}

func TestNoHarmNoFoul_FlagNorHarmCommaNoFowl(t *testing.T) {
	linttest.AssertSuggestionResult(t, "nor harm, no fowl", NewNoHarmNoFoul(), "no harm, no foul")
}

func TestNoHarmNoFoul_FlagNorHarmNorFoul(t *testing.T) {
	linttest.AssertSuggestionResult(t, "nor harm nor foul", NewNoHarmNoFoul(), "no harm no foul")
}

func TestNoHarmNoFoul_FlagNorHarmCommaNorFoul(t *testing.T) {
	linttest.AssertSuggestionResult(t, "nor harm, nor foul", NewNoHarmNoFoul(), "no harm, no foul")
}

func TestNoHarmNoFoul_FlagNorHarmNorFowl(t *testing.T) {
	linttest.AssertSuggestionResult(t, "nor harm nor fowl", NewNoHarmNoFoul(), "no harm no foul")
}

func TestNoHarmNoFoul_FlagNorHarmCommaNorFowl(t *testing.T) {
	linttest.AssertSuggestionResult(t, "nor harm, nor fowl", NewNoHarmNoFoul(), "no harm, no foul")
}

func TestNoHarmNoFoul_FixNoHarmNoFowlNoComma(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But no harm no fowl either way I now have proof that the experience of new users is not worth including a single word in a dependency list.", NewNoHarmNoFoul(), "But no harm no foul either way I now have proof that the experience of new users is not worth including a single word in a dependency list.")
}

func TestNoHarmNoFoul_FixNoHarmNoFowlWithComma(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I expect there will be no implementations that do otherwise, so no harm, no fowl, ugly corner cases go away....", NewNoHarmNoFoul(), "I expect there will be no implementations that do otherwise, so no harm, no foul, ugly corner cases go away....")
}

func TestNoHarmNoFoul_FixNoHarmNorFoulNoComma(t *testing.T) {
	linttest.AssertSuggestionResult(t, "and, yes, open source is a labor of love primarily so no harm nor foul when the goals take forever", NewNoHarmNoFoul(), "and, yes, open source is a labor of love primarily so no harm no foul when the goals take forever")
}

func TestNoHarmNoFoul_FixNoHarmNorFowlNoComma(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Thankfully my car bounced off and landed wheels down so no harm nor fowl", NewNoHarmNoFoul(), "Thankfully my car bounced off and landed wheels down so no harm no foul")
}

func TestNoHarmNoFoul_FixNorHarmNoFoulWithComma(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This happened today sorry not sorry to those that were scared \"scaleless\" it was too funny nor harm no foul", NewNoHarmNoFoul(), "This happened today sorry not sorry to those that were scared \"scaleless\" it was too funny no harm no foul")
}

func TestNoHarmNoFoul_FixNorHarmNoFowlWithComma(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She was mad. But I say nor harm, no fowl.", NewNoHarmNoFoul(), "She was mad. But I say no harm, no foul.")
}

func TestNoHarmNoFoul_FixNorHarmNorFoul(t *testing.T) {
	linttest.AssertSuggestionResult(t, "As long as the item was unharmed nor harm nor foul.", NewNoHarmNoFoul(), "As long as the item was unharmed no harm no foul.")
}
