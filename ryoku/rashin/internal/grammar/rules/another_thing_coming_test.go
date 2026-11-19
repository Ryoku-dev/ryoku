package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAnotherThingComing_FixHaveAnotherThinkComing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If you think that, you have another think coming, English.", NewAnotherThingComing(), "If you think that, you have another thing coming, English.")
}

func TestAnotherThingComing_FixHasAnotherThinkComing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If the wage earner thinks that he will obtain anything from either of the old parties he has another think coming.", NewAnotherThingComing(), "If the wage earner thinks that he will obtain anything from either of the old parties he has another thing coming.")
}

func TestAnotherThingComing_FixGotAnotherThinkComing(t *testing.T) {
	t.Skip("A lettercase bug results in 'another thiNG COMing.'")
	linttest.AssertSuggestionResult(t, "The correct phrase is, “You've got another THINK coming.”", NewAnotherThingComing(), "The correct phrase is, “You've got another THING coming.”")
}

func TestAnotherThingComing_FixHadAnotherThinkComing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Guess I had another think coming.", NewAnotherThingComing(), "Guess I had another thing coming.")
}
