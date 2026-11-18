package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestExpandIme_AfterCommaSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "foo bar, ime baz", NewExpandIme(), "foo bar, In my experience baz")
}

func TestExpandIme_AfterCommaNoSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "foo bar,ime baz", NewExpandIme(), "foo bar,In my experience baz")
}

func TestExpandIme_AfterFullStop(t *testing.T) {
	linttest.AssertSuggestionResult(t, "foo bar. ime baz", NewExpandIme(), "foo bar. In my experience baz")
}

func TestExpandIme_AfterFullStopNoSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "foo bar.ime baz", NewExpandIme(), "foo bar.In my experience baz")
}

func TestExpandIme_FixAtStartOfSentenceWithComma(t *testing.T) {
	linttest.AssertSuggestionResult(t, "IME, at least a weak of full-time dedication is needed, with a suitable editor and config, to start grasping this.", NewExpandIme(), "In my experience, at least a weak of full-time dedication is needed, with a suitable editor and config, to start grasping this.")
}

func TestExpandIme_FixAtStartOfSentenceWithoutComma(t *testing.T) {
	linttest.AssertSuggestionResult(t, "IME these types of tests are slow, brittle and don't give you much value.", NewExpandIme(), "In my experience these types of tests are slow, brittle and don't give you much value.")
}

func TestExpandIme_DontFlagMidChunk(t *testing.T) {
	linttest.AssertNoLints(t, "I have an IME that allows me to enter characters by their unicode code point.", NewExpandIme())
}
