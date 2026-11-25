package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestDeterminerWithoutNoun_FlagsDeterminerFollowedByConjunction(t *testing.T) {
	linttest.AssertLintCount(t, "The and other options were ignored.", NewDeterminerWithoutNoun(), 1)
}

func TestDeterminerWithoutNoun_FlagsIndefiniteArticleFollowedByConjunction(t *testing.T) {
	linttest.AssertLintCount(t, "A because I said so.", NewDeterminerWithoutNoun(), 1)
	linttest.AssertLintCount(t, "An because I said so.", NewDeterminerWithoutNoun(), 1)
}

func TestDeterminerWithoutNoun_AllowsCorrectUseWithNoun(t *testing.T) {
	linttest.AssertLintCount(t, "The dog barked.", NewDeterminerWithoutNoun(), 0)
}

func TestDeterminerWithoutNoun_AllowsDeterminerNounThenConjunction(t *testing.T) {
	linttest.AssertLintCount(t, "The dog and the cat played.", NewDeterminerWithoutNoun(), 0)
}
