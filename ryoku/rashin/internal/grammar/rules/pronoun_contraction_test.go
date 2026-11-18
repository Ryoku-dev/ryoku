package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestPronounContraction_Issue225(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Your the man", NewPronounContraction(), "You're the man")
}

func TestPronounContraction_WereTeam(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Were the best team.", NewPronounContraction(), "We're the best team.")
}

func TestPronounContraction_Issue139(t *testing.T) {
	linttest.AssertSuggestionResult(t, "it would be great if you're PR was merged into tower-lsp", NewPronounContraction(), "it would be great if your PR was merged into tower-lsp")
}

func TestPronounContraction_Car(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You're car is black.", NewPronounContraction(), "Your car is black.")
}

func TestPronounContraction_AllowsYouAreStill(t *testing.T) {
	linttest.AssertLintCount(t, "In case you're still not convinced.", NewPronounContraction(), 0)
}

func TestPronounContraction_Issue576(t *testing.T) {
	linttest.AssertLintCount(t, "If you're not happy you try again.", NewPronounContraction(), 0)
	linttest.AssertLintCount(t, "No you're not.", NewPronounContraction(), 0)
	linttest.AssertLintCount(t, "Even if you're not fluent in arm assembly, you surely noticed this.", NewPronounContraction(), 0)
}
