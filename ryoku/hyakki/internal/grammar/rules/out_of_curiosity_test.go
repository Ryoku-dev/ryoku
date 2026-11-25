package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestOutOfCuriosity_FixToSingular(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Just out of curiosities, do you know why it has a combo of super high and super low reviews?", NewOutOfCuriosity(), "Just out of curiosity, do you know why it has a combo of super high and super low reviews?")
}

func TestOutOfCuriosity_FixToPossessive(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Out of curiosities sake, I also tested on my Win10 machine with the WSL.", NewOutOfCuriosity(), "Out of curiosity's sake, I also tested on my Win10 machine with the WSL.")
}
