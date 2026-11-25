package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAnotherThinkComing_FixGotAnotherThingComing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If Microsoft thinks my Team and I are going to REINSTALL Windows fresh on over 1500 PC's they've got another thing coming!!", NewAnotherThinkComing(), "If Microsoft thinks my Team and I are going to REINSTALL Windows fresh on over 1500 PC's they've got another think coming!!")
}

func TestAnotherThinkComing_FixHasAnotherThingComing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Anyone who thinks it's easy to raise a child has another thing coming.", NewAnotherThinkComing(), "Anyone who thinks it's easy to raise a child has another think coming.")
}

func TestAnotherThinkComing_FixHaveAnotherThingComing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "And if you think they're predictable, you have another thing coming still.", NewAnotherThinkComing(), "And if you think they're predictable, you have another think coming still.")
}

func TestAnotherThinkComing_FixHadAnotherThingComing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "And wouldn't you know it I had another thing coming.", NewAnotherThinkComing(), "And wouldn't you know it I had another think coming.")
}
