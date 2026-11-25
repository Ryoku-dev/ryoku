package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestCant_CorrectsPronounCant(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I cant go to the store.", NewCant(), "I can't go to the store.")
}

func TestCant_CorrectsProperNounCant(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Bob cant go to the store.", NewCant(), "Bob can't go to the store.")
}

func TestCant_CorrectsCommonNounCant(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A horse cant drink bottled water.", NewCant(), "A horse can't drink bottled water.")
}

func TestCant_CorrectsCantPronoun(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Cant you go to the store?", NewCant(), "Can't you go to the store?")
}

func TestCant_DontFlagIfCantIsPartOfNounPhrase(t *testing.T) {
	linttest.AssertLintCount(t, "Cant cant be the same as jargon.", NewCant(), 0)
}

func TestCant_DontFlagCantProject(t *testing.T) {
	linttest.AssertLintCount(t, "The CANT project is designed to allow people to screw around with CAN easily at layers 1/2.", NewCant(), 0)
}

func TestCant_CorrectsCantVerb(t *testing.T) {
	t.Skip("Upstream ignored: 'Convert' is also a noun, so a 'cant convert' could be a person who switched to speaking jargon")
	linttest.AssertSuggestionResult(t, "Cant convert widget to input", NewCant(), "Can't convert widget to input")
}

func TestCant_DontFlagLegitNounSense(t *testing.T) {
	linttest.AssertLintCount(t, "CB Slang Dictionary is the distinctive anti-language, argot or cant which developed amongst users of citizens' band radio (CB), especially truck drivers", NewCant(), 0)
}
