package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestBeAllowed_CorrectsBasicSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You will not allowed to enter the lab.", NewBeAllowed(), "You will not be allowed to enter the lab.")
}

func TestBeAllowed_CorrectsFirstPersonSubject(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I will not allowed to go tonight.", NewBeAllowed(), "I will not be allowed to go tonight.")
}

func TestBeAllowed_CorrectsPluralSubject(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Students will not allowed to submit late work.", NewBeAllowed(), "Students will not be allowed to submit late work.")
}

func TestBeAllowed_CorrectsWithIntroClause(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Because of policy, workers will not allowed to take photos.", NewBeAllowed(), "Because of policy, workers will not be allowed to take photos.")
}

func TestBeAllowed_CorrectsContractedForm(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They won't allowed to park here during events.", NewBeAllowed(), "They won't be allowed to park here during events.")
}

func TestBeAllowed_CorrectsAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "THEY WILL NOT ALLOWED TO ENTER.", NewBeAllowed(), "THEY WILL NOT BE ALLOWED TO ENTER.")
}

func TestBeAllowed_CorrectsWithTrailingClause(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Without a permit, guests will not allowed to stay overnight at the cabin.", NewBeAllowed(), "Without a permit, guests will not be allowed to stay overnight at the cabin.")
}

func TestBeAllowed_CorrectsWithModalContext(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Even with approval, contractors will not allowed to access production.", NewBeAllowed(), "Even with approval, contractors will not be allowed to access production.")
}

func TestBeAllowed_LeavesCorrectPhraseUntouched(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They will not be allowed to park here during events.", NewBeAllowed(), "They will not be allowed to park here during events.")
}

func TestBeAllowed_LeavesOtherVerbsAlone(t *testing.T) {
	linttest.AssertLintCount(t, "We will not allow visitors after nine.", NewBeAllowed(), 0)
}

func TestBeAllowed_LeavesSimilarSequenceWithoutTo(t *testing.T) {
	linttest.AssertLintCount(t, "They won't be allowed to park here during events.", NewBeAllowed(), 0)
}
