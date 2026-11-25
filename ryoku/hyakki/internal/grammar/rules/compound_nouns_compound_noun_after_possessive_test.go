package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestCompoundNounsCompoundNounAfterPossessive_LetsIsNotPossessive(t *testing.T) {
	linttest.AssertLintCount(t, "Let's check out this article.", NewCompoundNounAfterPossessive(), 0)
}

func TestCompoundNounsCompoundNounAfterPossessive_LetsIsNotPossessiveTypographicApostrophe(t *testing.T) {
	linttest.AssertLintCount(t, "“Let’s go on with the game,” the Queen said to Alice;", NewCompoundNounAfterPossessive(), 0)
}

func TestCompoundNounsCompoundNounAfterPossessive_ThatsIsNotPossessive(t *testing.T) {
	linttest.AssertLintCount(t, "And you might not be thinking that that's a very big issue, but ...", NewCompoundNounAfterPossessive(), 0)
}
