package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestIfWouldve_FlagIfIWouldveDoneX(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If I would've done X...", NewIfWouldve(), "If I had done X...")
}

func TestIfWouldve_FlagIfYouWouldHaveDoneY(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If you would have done Y...", NewIfWouldve(), "If you had done Y...")
}

func TestIfWouldve_FlagIfWeWouldOfZ(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If we would of done Z...", NewIfWouldve(), "If we had done Z...")
}

func TestIfWouldve_FlagIfHeHadveDoneW(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If he hadve done W...", NewIfWouldve(), "If he had done W...")
}

func TestIfWouldve_FlagIfSheHadveDoneX(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If she had've done X...", NewIfWouldve(), "If she had done X...")
}

func TestIfWouldve_FlagIfItHadOfDoneX(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If it had of done X...", NewIfWouldve(), "If it had done X...")
}

func TestIfWouldve_FlagIfNpWouldve(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If that guy would've thought it through...", NewIfWouldve(), "If that guy had thought it through...")
}

func TestIfWouldve_DontFlagNonCounterfactualDone(t *testing.T) {
	t.Skip("Can't detect correct use not in counterfactual")
	linttest.AssertNoLints(t, "I don't know if they would have done that for a designer", NewIfWouldve())
}

func TestIfWouldve_DontFlagNonCounterfactualGotten(t *testing.T) {
	t.Skip("Can't detect correct use not in counterfactual")
	linttest.AssertNoLints(t, "I don't know if a normal programmer would have gotten that treatment.", NewIfWouldve())
}

func TestIfWouldve_DontFlagNonCounterfactualBeen(t *testing.T) {
	t.Skip("Can't detect correct use not in counterfactual")
	linttest.AssertNoLints(t, "I don't know if they would have been interested anyway", NewIfWouldve())
}
