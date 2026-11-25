package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAsap_CorrectsAsap(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Please respond asap.", NewAsSoonAsPossible(), "Please respond as soon as possible.")
}

func TestAsap_CorrectsAsapCapsAsPossible(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Regardless, let's get the deprecation in ASAP as possible to signal to users to stop using it.", NewAsSoonAsPossible(), []string{
		"Regardless, let's get the deprecation in as soon as possible to signal to users to stop using it.",
		"Regardless, let's get the deprecation in ASAP to signal to users to stop using it.",
	}, []string{})
}

func TestAsap_CorrectsAsapLowerAsPossible(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I need $1000 asap as possible", NewAsSoonAsPossible(), []string{"I need $1000 as soon as possible", "I need $1000 ASAP"}, []string{})
}

func TestAsap_CorrectsAsapAsPossibleAllCaps(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "WELL JUST TELL HIM TO CALL ME ASAP AS POSSIBLE.", NewAsSoonAsPossible(), []string{
		"WELL JUST TELL HIM TO CALL ME AS SOON AS POSSIBLE.",
		"WELL JUST TELL HIM TO CALL ME ASAP.",
	}, []string{})
}

func TestAsap_CorrectsAsAsapCapsAsPossible(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Please feel free just to send one of us a brief, descriptive email with your question, and we'll do our best to get back to you as ASAP as possible.", NewAsSoonAsPossible(), []string{
		"Please feel free just to send one of us a brief, descriptive email with your question, and we'll do our best to get back to you as soon as possible.",
		"Please feel free just to send one of us a brief, descriptive email with your question, and we'll do our best to get back to you ASAP.",
	}, []string{})
}

func TestAsap_CorrectsAsAsapLowerAsPossible(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "security updates should still be created as asap as possible.", NewAsSoonAsPossible(), []string{
		"security updates should still be created as soon as possible.",
		"security updates should still be created ASAP.",
	}, []string{})
}
