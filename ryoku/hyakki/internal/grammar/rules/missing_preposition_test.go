package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestMissingPreposition_FixesIssue1513(t *testing.T) {
	linttest.AssertLintCount(t, "The city is famous its beaches.", NewMissingPreposition(), 1)
	linttest.AssertLintCount(t, "The students are interested learning.", NewMissingPreposition(), 1)
}

func TestMissingPreposition_AllowsCorrectedIssue1513(t *testing.T) {
	linttest.AssertNoLints(t, "The city is famous for its beaches.", NewMissingPreposition())
	linttest.AssertNoLints(t, "The students are interested in learning.", NewMissingPreposition())
}

func TestMissingPreposition_NoLintWithoutAdjNounSequence(t *testing.T) {
	linttest.AssertLintCount(t, "She is happy.", NewMissingPreposition(), 0)
}

func TestMissingPreposition_NoLintWithPrepositionPresent(t *testing.T) {
	linttest.AssertLintCount(t, "They are fond of music.", NewMissingPreposition(), 0)
	linttest.AssertLintCount(t, "Students are interested in history.", NewMissingPreposition(), 0)
}

func TestMissingPreposition_FlagAdjPronPair(t *testing.T) {
	linttest.AssertLintCount(t, "He was angry him.", NewMissingPreposition(), 1)
}

func TestMissingPreposition_NoLintEmpty(t *testing.T) {
	linttest.AssertLintCount(t, "", NewMissingPreposition(), 0)
}

func TestMissingPreposition_AllowsTiredHerself(t *testing.T) {
	linttest.AssertNoLints(t, "She had tired herself out with trying.", NewMissingPreposition())
}

func TestMissingPreposition_AllowsTerribleStuff(t *testing.T) {
	linttest.AssertNoLints(t, "Either it was terrible stuff or the whiskey distorted things.", NewMissingPreposition())
}

func TestMissingPreposition_AllowsIssue1585(t *testing.T) {
	linttest.AssertNoLints(t, "Each agent has specific tools and tasks orchestrated through a crew workflow.", NewMissingPreposition())
}
