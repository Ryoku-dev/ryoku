package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestSingleBe_RemovesDoubleIs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The server is is slow.", NewSingleBe(), "The server is slow.")
}

func TestSingleBe_RemovesIsAre(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is are unusual.", NewSingleBe(), "This is unusual.")
}

func TestSingleBe_RemovesAreWereMismatch(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They are were excited.", NewSingleBe(), "They are excited.")
}

func TestSingleBe_RemovesMismatchedPair(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That is was odd.", NewSingleBe(), "That is odd.")
}

func TestSingleBe_HandlesSContraction(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The error's are gone.", NewSingleBe(), "The error's gone.")
}

func TestSingleBe_HandlesReContraction(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We're are ready to ship.", NewSingleBe(), "We're ready to ship.")
}

func TestSingleBe_HandlesMContraction(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm am aware.", NewSingleBe(), "I'm aware.")
}

func TestSingleBe_HandlesFutureRepetition(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That will be be an issue.", NewSingleBe(), "That will be an issue.")
}

func TestSingleBe_SkipsBeingChain(t *testing.T) {
	linttest.AssertNoLints(t, "It's been being rebuilt for months.", NewSingleBe())
}

func TestSingleBe_AllowsSimpleBeStatement(t *testing.T) {
	linttest.AssertNoLints(t, "Let's be honest.", NewSingleBe())
}

func TestSingleBe_AllowsPossessiveBeforeAre(t *testing.T) {
	linttest.AssertNoLints(t, "Stories like Mateo's are the heart of what we do.", NewSingleBe())
}

func TestSingleBe_RemovesAcrossNewline(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That is\nis tricky.", NewSingleBe(), "That is tricky.")
}

func TestSingleBe_IgnoresSeparatedForms(t *testing.T) {
	linttest.AssertNoLints(t, "The server is not is down.", NewSingleBe())
}

func TestSingleBe_IgnoresSingleBe(t *testing.T) {
	linttest.AssertNoLints(t, "This is ready.", NewSingleBe())
}

func TestSingleBe_DontFlagBeNounApostropheS(t *testing.T) {
	linttest.AssertNoLints(t, "Tiram seat believed to be DAP’s offer to Marina", NewSingleBe())
}
