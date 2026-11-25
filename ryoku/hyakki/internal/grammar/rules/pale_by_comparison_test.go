package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestPaleByComparison_FixArePaleIn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Admittedly, my R skills are pale in comparison to many of you guys, but this is a project I could undertake.", NewPaleByComparison(), "Admittedly, my R skills pale in comparison to many of you guys, but this is a project I could undertake.")
}

func TestPaleByComparison_FixBePaleIn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "their production would be pale in comparison to what we currently produce.", NewPaleByComparison(), "their production would pale in comparison to what we currently produce.")
}

func TestPaleByComparison_FixIsPaleBy(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The computed dollar value is pale by comparison.", NewPaleByComparison(), "The computed dollar value pales by comparison.")
}

func TestPaleByComparison_FixIsPaleIn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Humans have problem to grokk exponential growth which is pale in comparison to Ackermann function.", NewPaleByComparison(), "Humans have problem to grokk exponential growth which pales in comparison to Ackermann function.")
}

func TestPaleByComparison_FixWasPaleIn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "if you took the same actions to remove the violence and sexual fantasy out of Shakespeare, which was pale in comparison", NewPaleByComparison(), "if you took the same actions to remove the violence and sexual fantasy out of Shakespeare, which paled in comparison")
}

func TestPaleByComparison_FixWasPaleBy(t *testing.T) {
	linttest.AssertSuggestionResult(t, "What Microsoft did in the 90s was pale by comparison—they just bundled some extra software.", NewPaleByComparison(), "What Microsoft did in the 90s paled by comparison—they just bundled some extra software.")
}

func TestPaleByComparison_FixWerePaleIn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's not just that other experiences were pale in comparison to what she'd been doing, it's likely she was chemically incapable of feeling", NewPaleByComparison(), "It's not just that other experiences paled in comparison to what she'd been doing, it's likely she was chemically incapable of feeling")
}
