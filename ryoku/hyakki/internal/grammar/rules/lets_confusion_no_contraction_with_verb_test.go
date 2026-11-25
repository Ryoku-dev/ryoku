package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestLetsConfusionNoContractionWithVerb_FixLetsInspect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In the end lets inspect with git-blame the results.", NewNoContractionWithVerb(), "In the end let's inspect with git-blame the results.")
}

func TestLetsConfusionNoContractionWithVerb_DontFlagLetChance(t *testing.T) {
	linttest.AssertLintCount(t, "Let chance decide", NewNoContractionWithVerb(), 0)
}

func TestLetsConfusionNoContractionWithVerb_DontFlagLetTime(t *testing.T) {
	linttest.AssertLintCount(t, "Let time granularity be parametrized", NewNoContractionWithVerb(), 0)
}

func TestLetsConfusionNoContractionWithVerb_DontFlagLetsStaff(t *testing.T) {
	linttest.AssertLintCount(t, "A plugin that backs up player's inventories and lets staff restore them or export it as a shulker.", NewNoContractionWithVerb(), 0)
}

func TestLetsConfusionNoContractionWithVerb_DontFlagLetsTime(t *testing.T) {
	linttest.AssertLintCount(t, "This is very different than demo recording, which just simulates a network level connection and lets time move at its own rate.", NewNoContractionWithVerb(), 0)
}

func TestLetsConfusionNoContractionWithVerb_DontFlagLetsPlay(t *testing.T) {
	linttest.AssertLintCount(t, "Sometimes the umpire lets play continue", NewNoContractionWithVerb(), 0)
}

func TestLetsConfusionNoContractionWithVerb_DontFlagLetSleeping(t *testing.T) {
	linttest.AssertLintCount(t, "Let sleeping logs lie.", NewNoContractionWithVerb(), 0)
}

func TestLetsConfusionNoContractionWithVerb_DontFlagLetProcessed(t *testing.T) {
	linttest.AssertLintCount(t, "Let processed response be a new structure analogous to server auction response.", NewNoContractionWithVerb(), 0)
}

func TestLetsConfusionNoContractionWithVerb_CorrectsLetsMakeThis(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Lets make this joke repo into one of the best.", NewNoContractionWithVerb(), "Let's make this joke repo into one of the best.")
}

func TestLetsConfusionNoContractionWithVerb_CorrectsLetsMockThem(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Then lets mock them using Module._load based mocker.", NewNoContractionWithVerb(), "Then let's mock them using Module._load based mocker.")
}

func TestLetsConfusionNoContractionWithVerb_DontFlagLetUs(t *testing.T) {
	linttest.AssertLintCount(t, "Let us do this.", NewNoContractionWithVerb(), 0)
}

func TestLetsConfusionNoContractionWithVerb_DontFlagLetGo1202(t *testing.T) {
	linttest.AssertLintCount(t, "... until you hit your opponent, then let go and quickly retap", NewNoContractionWithVerb(), 0)
}

func TestLetsConfusionNoContractionWithVerb_DontFlagLetInAnd(t *testing.T) {
	linttest.AssertLintCount(t, "Japanese is good enough to be let in and.", NewNoContractionWithVerb(), 0)
}
