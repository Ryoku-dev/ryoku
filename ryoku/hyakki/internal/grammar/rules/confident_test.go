package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestConfident_DescribingPersonIncorrect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She felt confidant about her presentation.", NewConfident(), "She felt confident about her presentation.")
}

func TestConfident_DescribingPersonCorrect(t *testing.T) {
	linttest.AssertLintCount(t, "She felt confident about her presentation.", NewConfident(), 0)
}

func TestConfident_CertaintyIncorrect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am confidant the test results are accurate.", NewConfident(), "I am confident the test results are accurate.")
}

func TestConfident_CertaintyCorrect(t *testing.T) {
	linttest.AssertLintCount(t, "I am confident the test results are accurate.", NewConfident(), 0)
}

func TestConfident_DemeanorIncorrect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He walked to the stage with a confidant stride.", NewConfident(), "He walked to the stage with a confident stride.")
}

func TestConfident_DemeanorCorrect(t *testing.T) {
	linttest.AssertLintCount(t, "He walked to the stage with a confident stride.", NewConfident(), 0)
}

func TestConfident_ProfessionalIncorrect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You should sound confidant during job interviews.", NewConfident(), "You should sound confident during job interviews.")
}

func TestConfident_ProfessionalCorrect(t *testing.T) {
	linttest.AssertLintCount(t, "You should sound confident during job interviews.", NewConfident(), 0)
}

func TestConfident_AssuredToneIncorrect(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Present your argument in a confidant, persuasive manner.", NewConfident(), "Present your argument in a confident, persuasive manner.")
}

func TestConfident_AssuredToneCorrect(t *testing.T) {
	linttest.AssertLintCount(t, "Present your argument in a confident, persuasive manner.", NewConfident(), 0)
}

func TestConfident_ExtraTextBetween(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She felt very confidant about her presentation.", NewConfident(), "She felt very confident about her presentation.")
}

func TestConfident_LinkingVerbWasConfidant(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She was confidant about her presentation.", NewConfident(), "She was confident about her presentation.")
}
