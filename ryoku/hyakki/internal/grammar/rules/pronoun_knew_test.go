package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestPronounKnew_SimplePronounNew(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I new you would say that.", NewPronounKnew(), "I knew you would say that.")
}

func TestPronounKnew_WithAdverb(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She often new the answer.", NewPronounKnew(), "She often knew the answer.")
}

func TestPronounKnew_DoesNotFlagWithoutPronoun(t *testing.T) {
	linttest.AssertLintCount(t, "The software is new.", NewPronounKnew(), 0)
}

func TestPronounKnew_DoesNotFlagOtherContext(t *testing.T) {
	linttest.AssertLintCount(t, "They called it \"new\".", NewPronounKnew(), 0)
}

func TestPronounKnew_DoesNotFlagWithIts(t *testing.T) {
	linttest.AssertLintCount(t, "In 2015, the US was paying on average around 2% for its new issuance bonds.", NewPronounKnew(), 0)
}

func TestPronounKnew_DoesNotFlagWithHis(t *testing.T) {
	linttest.AssertLintCount(t, "His new car is fast.", NewPronounKnew(), 0)
}

func TestPronounKnew_DoesNotFlagWithHer(t *testing.T) {
	linttest.AssertLintCount(t, "Her new car is fast.", NewPronounKnew(), 0)
}

func TestPronounKnew_DoesNotFlagWithNothing1298(t *testing.T) {
	linttest.AssertLintCount(t, "This is nothing new.", NewPronounKnew(), 0)
}

func TestPronounKnew_DoesNotFlagWithAnyoneNew3257(t *testing.T) {
	linttest.AssertLintCount(t, "before inviting anyone new in.", NewPronounKnew(), 0)
}

func TestPronounKnew_DoesNotFlagWithIndefinitePronounsBeforeNew(t *testing.T) {
	linttest.AssertLintCount(t, "We need someone new for the role.", NewPronounKnew(), 0)
	linttest.AssertLintCount(t, "Everyone new should read the guide.", NewPronounKnew(), 0)
}

func TestPronounKnew_Issue1381Tricks(t *testing.T) {
	linttest.AssertLintCount(t, "To learn some new tricks.", NewPronounKnew(), 0)
}

func TestPronounKnew_Issue1381Template(t *testing.T) {
	linttest.AssertLintCount(t, "Let's build this new template function.", NewPronounKnew(), 0)
}

func TestPronounKnew_Issue1381File(t *testing.T) {
	linttest.AssertLintCount(t, "Move the function definition inside of that new file.", NewPronounKnew(), 0)
}

func TestPronounKnew_FixesIKnewWhat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I new what to do.", NewPronounKnew(), "I knew what to do.")
}

func TestPronounKnew_FixesSheKnewWhat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She new what to do.", NewPronounKnew(), "She knew what to do.")
}

func TestPronounKnew_FlagsSheNewDanger(t *testing.T) {
	linttest.AssertLintCount(t, "She new danger lurked nearby.", NewPronounKnew(), 1)
}

func TestPronounKnew_AllowsIssue1518(t *testing.T) {
	linttest.AssertNoLints(t, "If you're new to GitHub, welcome.", NewPronounKnew())
}
