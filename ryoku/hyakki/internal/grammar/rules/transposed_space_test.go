package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestTransposedSpace_SpaceTooEarly(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Th ecat sat on the mat.", NewTransposedSpaceSensitive(dict.Curated()), "The cat sat on the mat.")
}

func TestTransposedSpace_SpaceTooLate(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Thec at sat on the mat.", NewTransposedSpaceSensitive(dict.Curated()), "The cat sat on the mat.")
}

func TestTransposedSpace_TestEarly(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Sometimes the spac eis one character early.", NewTransposedSpace(dict.Curated()), "Sometimes the space is one character early.")
}

func TestTransposedSpace_TestLate(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Ands ometimes the space is a character late.", NewTransposedSpace(dict.Curated()), "And sometimes the space is a character late.")
}

func TestTransposedSpace_Test3355Result(t *testing.T) {
	linttest.AssertSuggestionResult(t, "cham peng", NewTransposedSpace(dict.Curated()), "champ eng")
}

func TestTransposedSpace_Test3355Count(t *testing.T) {
	linttest.AssertSuggestionCount(t, "cham peng", NewTransposedSpace(dict.Curated()), 1)
}
