package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestMassNouns_FlagAdvicesAndAnAdvice(t *testing.T) {
	linttest.AssertLintCount(t, "I asked for an advice. He gave me two advices!", NewMassNouns(dict.Curated()), 2)
}

func TestMassNouns_CorrectALuggage(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I managed to pack all my clothing into one luggage.", NewMassNouns(dict.Curated()), "I managed to pack all my clothing into one suitcase.")
}

func TestMassNouns_CorrectClothings(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I managed to pack all my clothings into one suitcase.", NewMassNouns(dict.Curated()), "I managed to pack all my clothing into one suitcase.")
}

func TestMassNouns_IgnoreAFunAndFantastic(t *testing.T) {
	linttest.AssertLintCount(t, "It was such a fun and fantastic adventure together that ...", NewMassNouns(dict.Curated()), 0)
}
