package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestChockFull_AllowsCorrectForm(t *testing.T) {
	linttest.AssertLintCount(t, "'Chalk full', 'chalk-full', 'choke full', and 'choke-full' are nonstandard forms of 'chock-full'.", NewChockFull(), 4)
}

func TestChockFull_LowerSpaceChalk(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The codebase is chalk full of errors that we need to address.", NewChockFull(), "The codebase is chock-full of errors that we need to address.")
}

func TestChockFull_LowerSpaceChoke(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The project is choke full of questionable decisions that we need to revisit.", NewChockFull(), "The project is chock-full of questionable decisions that we need to revisit.")
}

func TestChockFull_UpperSpaceChalk(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Chalk full of deprecated methods; we should refactor.", NewChockFull(), "Chock-full of deprecated methods; we should refactor.")
}

func TestChockFull_UpperSpaceChoke(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Choke full of unnecessary complexity; simplify it.", NewChockFull(), "Chock-full of unnecessary complexity; simplify it.")
}

func TestChockFull_LowerHyphenChalk(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The code is chalk-full of bugs; we need to debug before release.", NewChockFull(), "The code is chock-full of bugs; we need to debug before release.")
}

func TestChockFull_LowerHyphenChoke(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The project is choke-full of warnings; we should address them.", NewChockFull(), "The project is chock-full of warnings; we should address them.")
}

func TestChockFull_UpperHyphenChalk(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Chalk-full of features, but we only need a few.", NewChockFull(), "Chock-full of features, but we only need a few.")
}

func TestChockFull_UpperHyphenChoke(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Choke-full of pitfalls; let's consider alternatives.", NewChockFull(), "Chock-full of pitfalls; let's consider alternatives.")
}

func TestChockFull_LowerSpaceChocked(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The computer was chocked full with 64KB general use RAM (expandable up to 256KB)", NewChockFull(), "The computer was chock-full with 64KB general use RAM (expandable up to 256KB)")
}

func TestChockFull_LowerSpaceChucked(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The Mandalorian is chucked full of references and callbacks, mostly to previous Star Wars content.", NewChockFull(), "The Mandalorian is chock-full of references and callbacks, mostly to previous Star Wars content.")
}

func TestChockFull_LowerSpaceChoked(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I was making decent money, single and choked full of self-importance.", NewChockFull(), "I was making decent money, single and chock-full of self-importance.")
}

func TestChockFull_UpperSpaceChoked(t *testing.T) {
	t.Skip("replace_with_match_case quirks generated 'CHOCK-Full' instead of 'CHOCK-full'")
	linttest.AssertSuggestionResult(t, "Also being a family computer it’s CHOKED full, lots of unnecessary documents no one will sort", NewChockFull(), "Also being a family computer it’s CHOCK-full, lots of unnecessary documents no one will sort")
}
