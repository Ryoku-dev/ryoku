package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestMultipleSequentialPronouns_CanDetectTwoPronouns(t *testing.T) {
	linttest.AssertLintCount(t, "...little bit about my I want to do.", NewMultipleSequentialPronouns(), 1)
}

func TestMultipleSequentialPronouns_CanDetectThreePronouns(t *testing.T) {
	linttest.AssertLintCount(t, "...little bit about my I you want to do.", NewMultipleSequentialPronouns(), 1)
}

func TestMultipleSequentialPronouns_AllowsSinglePronouns(t *testing.T) {
	linttest.AssertLintCount(t, "...little bit about I want to do.", NewMultipleSequentialPronouns(), 0)
}

func TestMultipleSequentialPronouns_DetectsMultiplePronounsAtEnd(t *testing.T) {
	linttest.AssertLintCount(t, "...I need to explain this to you them.", NewMultipleSequentialPronouns(), 1)
}

func TestMultipleSequentialPronouns_CommaSeparated(t *testing.T) {
	linttest.AssertLintCount(t, "To prove it, we...", NewMultipleSequentialPronouns(), 0)
}

func TestMultipleSequentialPronouns_DontFlag578(t *testing.T) {
	linttest.AssertLintCount(t, "I can lend you my car.", NewMultipleSequentialPronouns(), 0)
}

func TestMultipleSequentialPronouns_DontFlag724(t *testing.T) {
	linttest.AssertLintCount(t, "One told me they were able to begin reading.", NewMultipleSequentialPronouns(), 0)
}

func TestMultipleSequentialPronouns_DontFlagUs(t *testing.T) {
	linttest.AssertLintCount(t, "Take the plunge and pull plug from their US tech.", NewMultipleSequentialPronouns(), 0)
}

func TestMultipleSequentialPronouns_DontFlagMyUsYourUs(t *testing.T) {
	linttest.AssertLintCount(t, "My US passport looks different from your US passport.", NewMultipleSequentialPronouns(), 0)
}

func TestMultipleSequentialPronouns_DontFlagSubjectAfterUsa(t *testing.T) {
	linttest.AssertLintCount(t, "And if it’s manufactured in the US it may have more automation.", NewMultipleSequentialPronouns(), 0)
}

func TestMultipleSequentialPronouns_DontFlagCaseInsensitiveCostHimHisLife(t *testing.T) {
	linttest.AssertLintCount(t, "to the point where it very well likely cost Him his life", NewMultipleSequentialPronouns(), 0)
}

func TestMultipleSequentialPronouns_DontFlag2870(t *testing.T) {
	linttest.AssertLintCount(t, "their sales derp was just having none of it when our IT director told him, point blank, that we're not moving anything into the cloud", NewMultipleSequentialPronouns(), 0)
}
