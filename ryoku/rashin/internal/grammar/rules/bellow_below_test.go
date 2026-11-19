package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestBellowBelow_CorrectBellowTheTable(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Going bellow the table", NewBellowBelow(), "Going below the table")
}

func TestBellowBelow_CorrectBellowTheSurface(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The submarine descended bellow the surface", NewBellowBelow(), "The submarine descended below the surface")
}

func TestBellowBelow_CorrectScrollBellowTheFold(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Scroll bellow the fold to continue reading", NewBellowBelow(), "Scroll below the fold to continue reading")
}

func TestBellowBelow_CorrectBellowTheirTarget(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Profits fell bellow their target this quarter", NewBellowBelow(), "Profits fell below their target this quarter")
}

func TestBellowBelow_CorrectBellowHisKnees(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The water only came up to just bellow his knees", NewBellowBelow(), "The water only came up to just below his knees")
}

func TestBellowBelow_CorrectBellowItsRim(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Fill the jar to just bellow its rim", NewBellowBelow(), "Fill the jar to just below its rim")
}

func TestBellowBelow_CorrectBellowTheStairs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She stored the boxes bellow the stairs", NewBellowBelow(), "She stored the boxes below the stairs")
}

func TestBellowBelow_CorrectSentenceInitialCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Bellow the winter average, the lake froze solid", NewBellowBelow(), "Below the winter average, the lake froze solid")
}

func TestBellowBelow_CorrectBellowAThreshold(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Do not let the pressure drop bellow a safe threshold", NewBellowBelow(), "Do not let the pressure drop below a safe threshold")
}

func TestBellowBelow_CorrectBellowDigit(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Readings bellow 5 were discarded as noise", NewBellowBelow(), "Readings below 5 were discarded as noise")
}

func TestBellowBelow_CorrectDescendedBellowDigit(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The divers descended bellow 30 meters", NewBellowBelow(), "The divers descended below 30 meters")
}

func TestBellowBelow_CorrectBellowSpelledNumber(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Keep the solution bellow five degrees", NewBellowBelow(), "Keep the solution below five degrees")
}

func TestBellowBelow_AllowVerbAtSentenceEnd(t *testing.T) {
	linttest.AssertNoLints(t, "He bellowed.", NewBellowBelow())
}

func TestBellowBelow_AllowVerbWithPrepositionalPhrase(t *testing.T) {
	linttest.AssertNoLints(t, "She bellows with rage.", NewBellowBelow())
}

func TestBellowBelow_AllowVerbWithAdverb(t *testing.T) {
	linttest.AssertNoLints(t, "Bulls bellow loudly during mating season.", NewBellowBelow())
}

func TestBellowBelow_AllowVerbWithDirectObject(t *testing.T) {
	linttest.AssertNoLints(t, "The sergeant bellowed orders across the parade ground.", NewBellowBelow())
}

func TestBellowBelow_AllowNounABellow(t *testing.T) {
	linttest.AssertNoLints(t, "He let out a bellow.", NewBellowBelow())
}

func TestBellowBelow_AllowNounPossessiveBellow(t *testing.T) {
	linttest.AssertNoLints(t, "The bull's bellow echoed across the valley.", NewBellowBelow())
}

func TestBellowBelow_AllowNounBellowsInstrument(t *testing.T) {
	linttest.AssertNoLints(t, "The blacksmith worked the bellows.", NewBellowBelow())
}

func TestBellowBelow_AllowInflectedBellowing(t *testing.T) {
	linttest.AssertNoLints(t, "The wind was bellowing all night.", NewBellowBelow())
}

func TestBellowBelow_AllowCorrectBelow(t *testing.T) {
	linttest.AssertNoLints(t, "Going below the table", NewBellowBelow())
}
