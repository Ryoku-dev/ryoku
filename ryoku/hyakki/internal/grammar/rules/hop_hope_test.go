package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestHopHope_CorrectsHopToHope(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I hop we can clarify this soon.", NewHopHope(), "I hope we can clarify this soon.")
}

func TestHopHope_DoesNotCorrectUnrelatedUse(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I hop on one foot for fun.", NewHopHope(), "I hop on one foot for fun.")
}

func TestHopHope_CorrectsMixedCaseHop(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I HoP we can find a solution.", NewHopHope(), "I HoPE we can find a solution.")
}

func TestHopHope_CorrectsHopingOnCall(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I was hoping on a call to discuss this.", NewHopHope(), "I was hopping on a call to discuss this.")
}

func TestHopHope_CorrectsHopedOnPlane(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She hoped on an airplane to visit family.", NewHopHope(), "She hopped on an airplane to visit family.")
}

func TestHopHope_CorrectsHopeOnBus(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They hope on a bus every morning.", NewHopHope(), "They hop on a bus every morning.")
}

func TestHopHope_DoesNotCorrectUnrelatedContext(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I hope everything goes well with your project.", NewHopHope(), "I hope everything goes well with your project.")
}

func TestHopHope_CorrectsMixedCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She HoPeD on a train to get home.", NewHopHope(), "She HoPpED on a train to get home.")
}
