package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestCommaFixes_AllowsEnglishCommaAtomic(t *testing.T) {
	linttest.AssertLintCount(t, ",", NewCommaFixes(), 0)
}

func TestCommaFixes_FlagsFullwidthCommaAtomic(t *testing.T) {
	linttest.AssertLintCount(t, "，", NewCommaFixes(), 1)
}

func TestCommaFixes_FlagsIdeographicCommaAtomic(t *testing.T) {
	linttest.AssertLintCount(t, "、", NewCommaFixes(), 1)
}

func TestCommaFixes_CorrectsFullwidthCommaRealWorld(t *testing.T) {
	linttest.AssertSuggestionResult(t, "higher 2 bits of the number of nodes， whether abandoned or not decided by .index section", NewCommaFixes(), "higher 2 bits of the number of nodes, whether abandoned or not decided by .index section")
}

func TestCommaFixes_CorrectsIdeographicCommaRealWorld(t *testing.T) {
	linttest.AssertSuggestionResult(t, "cout、endl、string", NewCommaFixes(), "cout, endl, string")
}

func TestCommaFixes_DoesntFlagCommaSpaceBetweenWords(t *testing.T) {
	linttest.AssertLintCount(t, "foo, bar", NewCommaFixes(), 0)
}

func TestCommaFixes_FlagsFullwidthCommaSpaceBetweenWords(t *testing.T) {
	linttest.AssertLintCount(t, "foo， bar", NewCommaFixes(), 1)
}

func TestCommaFixes_FlagsIdeographicCommaSpaceBetweenWords(t *testing.T) {
	linttest.AssertLintCount(t, "foo、 bar", NewCommaFixes(), 1)
}

func TestCommaFixes_DoesntFlagSemicolonSpaceBetweenWords(t *testing.T) {
	linttest.AssertLintCount(t, "foo; bar", NewCommaFixes(), 0)
}

func TestCommaFixes_CorrectsCommaBetweenWordsWithNoSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "foo,bar", NewCommaFixes(), "foo, bar")
}

func TestCommaFixes_CorrectsAsianCommaBetweenWordsWithNoSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "foo，bar", NewCommaFixes(), "foo, bar")
}

func TestCommaFixes_CorrectsSpaceOnWrongSideOfCommaBetweenWords(t *testing.T) {
	linttest.AssertSuggestionResult(t, "foo ,bar", NewCommaFixes(), "foo, bar")
}

func TestCommaFixes_CorrectsCommaOnWrongSideOfAsianCommaBetweenWords(t *testing.T) {
	linttest.AssertSuggestionResult(t, "foo ，bar", NewCommaFixes(), "foo, bar")
}

func TestCommaFixes_CorrectsCommaBetweenWordsWithSpaceOnBothSides(t *testing.T) {
	linttest.AssertSuggestionResult(t, "foo , bar", NewCommaFixes(), "foo, bar")
}

func TestCommaFixes_CorrectsAsianCommaBetweenWordsWithSpaceOnBothSides(t *testing.T) {
	linttest.AssertSuggestionResult(t, "foo 、 bar", NewCommaFixes(), "foo, bar")
}

func TestCommaFixes_DoesntCorrectCommaBetweenNonEnglishTokens(t *testing.T) {
	linttest.AssertLintCount(t, "严禁采摘花、 果、叶，挖掘树根、草药!", NewCommaFixes(), 0)
}

func TestCommaFixes_Issue2233(t *testing.T) {
	linttest.AssertNoLints(t, "In foobar, apple is a fruit, and \"beer\" is not a fruit.", NewCommaFixes())
}
