package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestRedundantFirsts_FisFirstCoined(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Slopsquatting, as researchers are calling it, is a term first coined by Seth Larson, a security developer-in-residence at Python Software Foundation (PSF), for its resemblance to the typosquatting technique.", NewRedundantFirsts(), "Slopsquatting, as researchers are calling it, is a term coined by Seth Larson, a security developer-in-residence at Python Software Foundation (PSF), for its resemblance to the typosquatting technique.")
}

func TestRedundantFirsts_FixFirstInventedSolution(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Ben Kamens has a great writeup about a version of the solution, first invented by Bruce Tognazzini in the late 80's.", NewRedundantFirsts(), "Ben Kamens has a great writeup about a version of the solution, invented by Bruce Tognazzini in the late 80's.")
}

func TestRedundantFirsts_FixFirstDiscovered(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I first discovered this bug when installing dependencies for navigator and import it on my App.js.", NewRedundantFirsts(), "I discovered this bug when installing dependencies for navigator and import it on my App.js.")
}

func TestRedundantFirsts_FixFirstIntroduced(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The Transformer architecture is a neural network architecture that was first introduced in 2016.", NewRedundantFirsts(), "The Transformer architecture is a neural network architecture that was introduced in 2016.")
}

func TestRedundantFirsts_FixFirstReleased(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This was supposed to revitalize the Tony Hawk franchise. It was first released in 2009.", NewRedundantFirsts(), "This was supposed to revitalize the Tony Hawk franchise. It was released in 2009.")
}

func TestRedundantFirsts_FixFirstInventedC(t *testing.T) {
	linttest.AssertSuggestionResult(t, "C was first invented back in 1972 at Bell Labs.", NewRedundantFirsts(), "C was invented back in 1972 at Bell Labs.")
}

func TestRedundantFirsts_FixFirstOriginated(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The Japanese Foreign Ministry, where the rumors first originated, said it still needed to check the reports but were not able to verify them.", NewRedundantFirsts(), "The Japanese Foreign Ministry, where the rumors originated, said it still needed to check the reports but were not able to verify them.")
}

func TestRedundantFirsts_FixDiscoveredForTheFirstTime(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've just discovered Harper for the first time, but already struck a number of issues, which I have raised in GitHub", NewRedundantFirsts(), "I've just discovered Harper, but already struck a number of issues, which I have raised in GitHub")
}

func TestRedundantFirsts_FixFirstFounded(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Gröna Lund (a name which translates to “Green Grove”) was first founded in 1883, when German showman Jacob Schultheis rented a patch of land on the waterfront in Stockholm.", NewRedundantFirsts(), "Gröna Lund (a name which translates to “Green Grove”) was founded in 1883, when German showman Jacob Schultheis rented a patch of land on the waterfront in Stockholm.")
}
