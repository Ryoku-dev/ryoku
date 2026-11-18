package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestHelloGreeting_CorrectsBasicGreeting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Halo John!", NewHelloGreeting(), "Hello John!")
}

func TestHelloGreeting_CorrectsWithComma(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Halo, Jane.", NewHelloGreeting(), "Hello, Jane.")
}

func TestHelloGreeting_CorrectsWithWorld(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Halo world!", NewHelloGreeting(), "Hello world!")
}

func TestHelloGreeting_CorrectsWithoutPunctuation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Halo there friend.", NewHelloGreeting(), "Hello there friend.")
}

func TestHelloGreeting_CorrectsSingleWordSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Halo!", NewHelloGreeting(), "Hello!")
}

func TestHelloGreeting_CorrectsQuestion(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Halo?", NewHelloGreeting(), "Hello?")
}

func TestHelloGreeting_CorrectsUppercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "HALO!", NewHelloGreeting(), "HELLO!")
}

func TestHelloGreeting_NoLintForOpticalTerm(t *testing.T) {
	linttest.AssertLintCount(t, "The halo around the moon glowed softly.", NewHelloGreeting(), 0)
}

func TestHelloGreeting_NoLintMidSentence(t *testing.T) {
	linttest.AssertLintCount(t, "They shouted hello, not Halo, during rehearsal.", NewHelloGreeting(), 0)
}

func TestHelloGreeting_CorrectsInQuotes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "\"Halo John!\"", NewHelloGreeting(), "\"Hello John!\"")
}
