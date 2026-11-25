package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestNominalWants_FixesHeWonts(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He wonts to join us.", NewNominalWants(), "He wants to join us.")
}

func TestNominalWants_FixesItWont(t *testing.T) {
	t.Skip("This is not a grammar error if the previous word is `help`, `let`, or `make`.")
	linttest.AssertSuggestionResult(t, "It wont to move forward.", NewNominalWants(), "It wants to move forward.")
}

func TestNominalWants_FixesSheWont(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She wont to leave early.", NewNominalWants(), "She wants to leave early.")
}

func TestNominalWants_FixesIWont(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I wonts to leave early.", NewNominalWants(), "I want to leave early.")
}

func TestNominalWants_AllowsYouWant(t *testing.T) {
	linttest.AssertLintCount(t, "What size do you want to be?", NewNominalWants(), 0)
}

func TestNominalWants_FixesYouWants(t *testing.T) {
	linttest.AssertSuggestionResult(t, "What do you wants?", NewNominalWants(), "What do you want?")
}

func TestNominalWants_IgnoresCorrectUsageThey(t *testing.T) {
	linttest.AssertLintCount(t, "They want to help.", NewNominalWants(), 0)
}

func TestNominalWants_IgnoresCorrectUsageHe(t *testing.T) {
	linttest.AssertLintCount(t, "He wants to help.", NewNominalWants(), 0)
}

func TestNominalWants_IgnoresCorrectUsageThat1298(t *testing.T) {
	linttest.AssertLintCount(t, "The projects that want to take it seriously are the best.", NewNominalWants(), 0)
}

func TestNominalWants_IgnoresCorrectUsageMakeMe(t *testing.T) {
	linttest.AssertLintCount(t, "Take another person code make me want to die.", NewNominalWants(), 0)
}

func TestNominalWants_IgnoresCorrectUsageMakesMe(t *testing.T) {
	linttest.AssertLintCount(t, "It makes me want to not use GitHub at all.", NewNominalWants(), 0)
}

func TestNominalWants_IgnoresCorrectUsageMakeUs(t *testing.T) {
	linttest.AssertLintCount(t, "... try harder to make us want to implement it.", NewNominalWants(), 0)
}

func TestNominalWants_IgnoresCorrectUsageMadeUs(t *testing.T) {
	linttest.AssertLintCount(t, "This change made us want to adopt luxon's strict mode", NewNominalWants(), 0)
}

func TestNominalWants_IgnoresCorrectUsageHelpUs(t *testing.T) {
	linttest.AssertLintCount(t, "... help us want to help you.", NewNominalWants(), 0)
}

func TestNominalWants_IgnoresCorrectUsageMakeYou(t *testing.T) {
	linttest.AssertLintCount(t, "I can certainly see why that would make you want to ditch Linux packaging.", NewNominalWants(), 0)
}

func TestNominalWants_IgnoresCorrectUsageMakesYou(t *testing.T) {
	linttest.AssertLintCount(t, "If something happens that makes you want to scream from the top of your lungs", NewNominalWants(), 0)
}

func TestNominalWants_IgnoresCorrectUsageMadeYou(t *testing.T) {
	linttest.AssertLintCount(t, "What made you want to leave the LibFuzzer ...", NewNominalWants(), 0)
}

func TestNominalWants_IgnoresCorrectUsageMakeHim(t *testing.T) {
	linttest.AssertLintCount(t, "make him want to help with your issue", NewNominalWants(), 0)
}

func TestNominalWants_IgnoresCorrectUsageMakeHer(t *testing.T) {
	linttest.AssertLintCount(t, "... and make her want to get into coding.", NewNominalWants(), 0)
}

func TestNominalWants_IgnoresCorrectUsageMakeIt(t *testing.T) {
	linttest.AssertLintCount(t, "you just make it want to appear as a drama", NewNominalWants(), 0)
}

func TestNominalWants_IgnoresCorrectUsageMakesIt(t *testing.T) {
	linttest.AssertLintCount(t, "using UHD makes it want to put labels in the corner saying UHD", NewNominalWants(), 0)
}

func TestNominalWants_IgnoresCorrectUsageMakeThem(t *testing.T) {
	linttest.AssertLintCount(t, "And make them want to believe in it.", NewNominalWants(), 0)
}

func TestNominalWants_IgnoresCorrectUsageMakingThem(t *testing.T) {
	linttest.AssertLintCount(t, "you're annoying ALMOST ALL of the users and making them want to switch to another ...", NewNominalWants(), 0)
}

func TestNominalWants_IgnoresCorrectUsageHelpThem(t *testing.T) {
	linttest.AssertLintCount(t, "And help them want to do it.", NewNominalWants(), 0)
}

func TestNominalWants_AllowsWantTo(t *testing.T) {
	linttest.AssertNoLints(t, "Harper is a grammar checker for people who want to write fast.", NewNominalWants())
}

func TestNominalWants_Test2007(t *testing.T) {
	linttest.AssertNoLints(t, "### 🙌 **We Want to Hear From You!**", NewNominalWants())
}
