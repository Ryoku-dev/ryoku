package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestWidelyAccepted_WideAcceptedLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It is wide accepted that exercise improves health.", NewWidelyAccepted(), "It is widely accepted that exercise improves health.")
}

func TestWidelyAccepted_WideAcceptableMixedCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Wide acceptable standards are used in the design.", NewWidelyAccepted(), "Widely acceptable standards are used in the design.")
}

func TestWidelyAccepted_WidelyAlreadyCorrect(t *testing.T) {
	linttest.AssertLintCount(t, "It is widely accepted that sunlight is beneficial in moderation.", NewWidelyAccepted(), 0)
}

func TestWidelyAccepted_NoFalsePositive(t *testing.T) {
	linttest.AssertLintCount(t, "The house had wide open windows during the renovation.", NewWidelyAccepted(), 0)
}

func TestWidelyAccepted_WideAcceptedInLongText(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is an example paragraph, and it is wide accepted that these changes will improve performance. In fact, widely used frameworks have already adopted them.", NewWidelyAccepted(), "This is an example paragraph, and it is widely accepted that these changes will improve performance. In fact, widely used frameworks have already adopted them.")
}

func TestWidelyAccepted_WideTwiceInOneSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It is wide accepted and wide used by many professionals.", NewWidelyAccepted(), "It is widely accepted and widely used by many professionals.")
}
