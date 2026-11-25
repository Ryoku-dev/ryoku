package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAskNoPreposition_FlagsAsk(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Nora asked to us about the concert lineup.", NewAskNoPreposition(), "Nora asked us about the concert lineup.")
}

func TestAskNoPreposition_FlagsAskAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "NORA ASKED TO US ABOUT THE CONCERT LINEUP.", NewAskNoPreposition(), "NORA ASKED US ABOUT THE CONCERT LINEUP.")
}

func TestAskNoPreposition_FlagsTell(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Please tell to him the results promptly.", NewAskNoPreposition(), "Please tell him the results promptly.")
}

func TestAskNoPreposition_IgnoresCorrectUsage(t *testing.T) {
	linttest.AssertLintCount(t, "She asked her mentor a difficult question.", NewAskNoPreposition(), 0)
}

func TestAskNoPreposition_FlagsAskUs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Can you ask to us for directions?", NewAskNoPreposition(), "Can you ask us for directions?")
}

func TestAskNoPreposition_FlagsAsksHim(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Julia asks to him every morning about the report.", NewAskNoPreposition(), "Julia asks him every morning about the report.")
}

func TestAskNoPreposition_FlagsAskedMe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They asked to me why I left early.", NewAskNoPreposition(), "They asked me why I left early.")
}

func TestAskNoPreposition_FlagsToldOne(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The guide told to one the secret path.", NewAskNoPreposition(), "The guide told one the secret path.")
}

func TestAskNoPreposition_FlagsTellingIt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She is telling to it with gentle words.", NewAskNoPreposition(), "She is telling it with gentle words.")
}

func TestAskNoPreposition_FlagsTellsThem(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He tells to them stories at night.", NewAskNoPreposition(), "He tells them stories at night.")
}

func TestAskNoPreposition_FlagsTellingHim(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I was telling to him the latest news.", NewAskNoPreposition(), "I was telling him the latest news.")
}

func TestAskNoPreposition_FlagsAskingYou(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Someone is asking to you for help.", NewAskNoPreposition(), "Someone is asking you for help.")
}

func TestAskNoPreposition_IgnoresAskQuestion(t *testing.T) {
	linttest.AssertLintCount(t, "Ask her the question directly.", NewAskNoPreposition(), 0)
}

func TestAskNoPreposition_IgnoresToldToLeave(t *testing.T) {
	linttest.AssertLintCount(t, "He was told to leave immediately.", NewAskNoPreposition(), 0)
}

func TestAskNoPreposition_IgnoresTellUs(t *testing.T) {
	linttest.AssertLintCount(t, "Please tell us your name.", NewAskNoPreposition(), 0)
}

func TestAskNoPreposition_IgnoresAskAbout(t *testing.T) {
	linttest.AssertLintCount(t, "They asked about the schedule.", NewAskNoPreposition(), 0)
}
