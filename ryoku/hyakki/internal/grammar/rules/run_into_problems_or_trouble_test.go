package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestRunIntoProblemsOrTrouble_RanIntoATrouble(t *testing.T) {
	linttest.AssertSuggestionResult(t, "but recently I ran into a trouble that the release button is not shown", NewRunIntoProblemsOrTrouble(), "but recently I ran into trouble that the release button is not shown")
}

func TestRunIntoProblemsOrTrouble_RanIntoProblem(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Ran into problem when trying to install the upgrade assistant", NewRunIntoProblemsOrTrouble(), []string{
		"Ran into problems when trying to install the upgrade assistant",
		"Ran into a problem when trying to install the upgrade assistant",
	}, []string{})
}

func TestRunIntoProblemsOrTrouble_RanIntoTroubles(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I ran into troubles when executing contrib/utilities/update_all_files.sh", NewRunIntoProblemsOrTrouble(), "I ran into trouble when executing contrib/utilities/update_all_files.sh")
}

func TestRunIntoProblemsOrTrouble_RunIntoATrouble(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm have run into a trouble when writing a lexer with below tokens", NewRunIntoProblemsOrTrouble(), "I'm have run into trouble when writing a lexer with below tokens")
}

func TestRunIntoProblemsOrTrouble_RunIntoProblem(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Running into problem when I add it into chatbox.", NewRunIntoProblemsOrTrouble(), []string{
		"Running into problems when I add it into chatbox.",
		"Running into a problem when I add it into chatbox.",
	}, []string{})
}

func TestRunIntoProblemsOrTrouble_RunIntoTroubles(t *testing.T) {
	linttest.AssertSuggestionResult(t, "[post here if you run into troubles]", NewRunIntoProblemsOrTrouble(), "[post here if you run into trouble]")
}

func TestRunIntoProblemsOrTrouble_RunningIntoATrouble(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We are spawning built app without this argument, thus running into a trouble in v3.5.0", NewRunIntoProblemsOrTrouble(), "We are spawning built app without this argument, thus running into trouble in v3.5.0")
}

func TestRunIntoProblemsOrTrouble_RunningIntoProblem(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Running into problem with Prisma when using Git submodules.", NewRunIntoProblemsOrTrouble(), []string{
		"Running into problems with Prisma when using Git submodules.",
		"Running into a problem with Prisma when using Git submodules.",
	}, []string{})
}

func TestRunIntoProblemsOrTrouble_RunningIntoTroubles(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Running into troubles with 0.6.2", NewRunIntoProblemsOrTrouble(), "Running into trouble with 0.6.2")
}

func TestRunIntoProblemsOrTrouble_RunsIntoProblem(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Serving FaceNet with Tensorflow Serving runs into problem with PyFunc", NewRunIntoProblemsOrTrouble(), []string{
		"Serving FaceNet with Tensorflow Serving runs into problems with PyFunc",
		"Serving FaceNet with Tensorflow Serving runs into a problem with PyFunc",
	}, []string{})
}

func TestRunIntoProblemsOrTrouble_RunIntoFurtherTroubles(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Let me know if you run into further troubles!", NewRunIntoProblemsOrTrouble(), "Let me know if you run into further trouble!")
}
