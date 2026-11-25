package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestNumericRangeEnDash_CorrectsBasicPageRange(t *testing.T) {
	linttest.AssertSuggestionResult(t, "See pages 12-14 for the full table.", NewNumericRangeEnDash(), "See pages 12–14 for the full table.")
}

func TestNumericRangeEnDash_CorrectsBasicEmDashRange(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Read chapters 3—5 before class.", NewNumericRangeEnDash(), "Read chapters 3–5 before class.")
}

func TestNumericRangeEnDash_CorrectsScoreRange(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The final score was 10-8 after overtime.", NewNumericRangeEnDash(), "The final score was 10–8 after overtime.")
}

func TestNumericRangeEnDash_CorrectsDecimalRange(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Keep the ratio between 1.5-2.5 during calibration.", NewNumericRangeEnDash(), "Keep the ratio between 1.5–2.5 during calibration.")
}

func TestNumericRangeEnDash_CorrectsPercentRange(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Expect a 5-10% improvement after tuning.", NewNumericRangeEnDash(), "Expect a 5–10% improvement after tuning.")
}

func TestNumericRangeEnDash_CorrectsYearSpan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The archive covers 1990-1995.", NewNumericRangeEnDash(), "The archive covers 1990–1995.")
}

func TestNumericRangeEnDash_CorrectsZeroPaddedRange(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Use files 01-03 for the demo.", NewNumericRangeEnDash(), "Use files 01–03 for the demo.")
}

func TestNumericRangeEnDash_CorrectsSentenceFinalRange(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Valid values are 2-4.", NewNumericRangeEnDash(), "Valid values are 2–4.")
}

func TestNumericRangeEnDash_IgnoresExistingEnDash(t *testing.T) {
	linttest.AssertNoLints(t, "See pages 12–14 for the full table.", NewNumericRangeEnDash())
}

func TestNumericRangeEnDash_IgnoresSpacedHyphenRange(t *testing.T) {
	linttest.AssertNoLints(t, "See pages 12 - 14 for the full table.", NewNumericRangeEnDash())
}

func TestNumericRangeEnDash_IgnoresSpacedEmDashRange(t *testing.T) {
	linttest.AssertNoLints(t, "See pages 12 — 14 for the full table.", NewNumericRangeEnDash())
}

func TestNumericRangeEnDash_IgnoresHyphenatedDateChain(t *testing.T) {
	linttest.AssertNoLints(t, "Today is 2026-03-18.", NewNumericRangeEnDash())
}

func TestNumericRangeEnDash_IgnoresEmDashDateChain(t *testing.T) {
	linttest.AssertNoLints(t, "Today is 2026—03—18.", NewNumericRangeEnDash())
}

func TestNumericRangeEnDash_IgnoresThreePartVersionChain(t *testing.T) {
	linttest.AssertNoLints(t, "The build number is 1-2-3.", NewNumericRangeEnDash())
}

func TestNumericRangeEnDash_IgnoresMixedDashChain(t *testing.T) {
	linttest.AssertNoLints(t, "The timeline reads 1-2—3.", NewNumericRangeEnDash())
}

func TestNumericRangeEnDash_IgnoresLongerNumericChain(t *testing.T) {
	linttest.AssertNoLints(t, "The code spans 12-14-16 in the export.", NewNumericRangeEnDash())
}

func TestNumericRangeEnDash_DontFlagCve3580(t *testing.T) {
	linttest.AssertNoLints(t, "CVE-2017-5753", NewNumericRangeEnDash())
}
