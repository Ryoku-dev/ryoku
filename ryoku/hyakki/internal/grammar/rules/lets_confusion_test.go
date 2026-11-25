package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestLetsConfusion_Walking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The crutch let's him walk.", NewLetsConfusion(), "The crutch lets him walk.")
}

func TestLetsConfusion_Issue426Us(t *testing.T) {
	linttest.AssertSuggestionResult(t, "let's us do", NewLetsConfusion(), "lets us do")
}

func TestLetsConfusion_Issue426Me(t *testing.T) {
	linttest.AssertSuggestionResult(t, "let's me do", NewLetsConfusion(), "lets me do")
}

func TestLetsConfusion_FromHarperDocs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Often the longest and the shortest words are the most helpful, so lets push them first.", NewLetsConfusion(), "Often the longest and the shortest words are the most helpful, so let's push them first.")
}

func TestLetsConfusion_Issue470MissingApostrophePlay(t *testing.T) {
	t.Skip("\"play\" is also a noun so in a context like \"Sometimes the umpire lets play continue\"")
	linttest.AssertSuggestionResult(t, "lets play", NewLetsConfusion(), "let's play")
}

func TestLetsConfusion_Issue470MissingSubjectPlay(t *testing.T) {
	t.Skip("Ignored upstream.")
	linttest.AssertSuggestionResult(t, "let play", NewLetsConfusion(), "let's play")
}

func TestLetsConfusion_Issue470MissingApostropheProceed(t *testing.T) {
	linttest.AssertSuggestionResult(t, "lets proceed", NewLetsConfusion(), "let's proceed")
}

func TestLetsConfusion_Issue470MissingSubjectProceed(t *testing.T) {
	linttest.AssertSuggestionResult(t, "let proceed", NewLetsConfusion(), "let's proceed")
}

func TestLetsConfusion_Issue548(t *testing.T) {
	linttest.AssertLintCount(t, "A simple web app that lets you fetch random issues.", NewLetsConfusion(), 0)
}

func TestLetsConfusion_Issue4037All(t *testing.T) {
	linttest.AssertLintCount(t, "Let's all go home.", NewLetsConfusion(), 0)
}

func TestLetsConfusion_Issue4037Both(t *testing.T) {
	linttest.AssertLintCount(t, "Let's both try again.", NewLetsConfusion(), 0)
}

func TestLetsConfusion_Issue4037Everyone(t *testing.T) {
	linttest.AssertLintCount(t, "Let's everyone take a seat.", NewLetsConfusion(), 0)
}

func TestLetsConfusion_Issue4037Everybody(t *testing.T) {
	linttest.AssertLintCount(t, "Let's everybody take a seat.", NewLetsConfusion(), 0)
}
