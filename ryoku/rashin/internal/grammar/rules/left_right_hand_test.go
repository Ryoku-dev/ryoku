package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestLeftRightHand_CorrectsLeftHandSide(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You'll see it on the left hand side.", NewLeftRightHand(), "You'll see it on the left-hand side.")
}

func TestLeftRightHand_CorrectsRightHandCorner(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's in the right hand corner.", NewLeftRightHand(), "It's in the right-hand corner.")
}

func TestLeftRightHand_DoesNotCorrectNounUsage(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She raised her right hand.", NewLeftRightHand(), "She raised her right hand.")
}
