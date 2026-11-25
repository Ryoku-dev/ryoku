package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestPronounInflectionBe_CorrectsHeAre(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He are my best friend.", NewPronounInflectionBe(), "He is my best friend.")
}

func TestPronounInflectionBe_CorrectsSheAre(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She are my best friend.", NewPronounInflectionBe(), "She is my best friend.")
}

func TestPronounInflectionBe_CorrectsTheyIs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They is my best friend.", NewPronounInflectionBe(), "They are my best friend.")
}

func TestPronounInflectionBe_AllowsTheyAre(t *testing.T) {
	linttest.AssertLintCount(t, "They are my best friend.", NewPronounInflectionBe(), 0)
}

func TestPronounInflectionBe_CorrectsItAre(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It are on the table.", NewPronounInflectionBe(), "It is on the table.")
}

func TestPronounInflectionBe_CorrectsHeAreNegation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He are not amused.", NewPronounInflectionBe(), "He is not amused.")
}

func TestPronounInflectionBe_CorrectsSheAreProgressive(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She are going to win.", NewPronounInflectionBe(), "She is going to win.")
}

func TestPronounInflectionBe_CorrectsTheyIsNegation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They is not ready.", NewPronounInflectionBe(), "They are not ready.")
}

func TestPronounInflectionBe_CorrectsTheyIsProgressive(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They is planning a trip.", NewPronounInflectionBe(), "They are planning a trip.")
}

func TestPronounInflectionBe_AllowsHeIs(t *testing.T) {
	linttest.AssertLintCount(t, "He is my best friend.", NewPronounInflectionBe(), 0)
}

func TestPronounInflectionBe_AllowsSheIsLowercase(t *testing.T) {
	linttest.AssertLintCount(t, "she is excited to go.", NewPronounInflectionBe(), 0)
}

func TestPronounInflectionBe_AllowsItIs(t *testing.T) {
	linttest.AssertLintCount(t, "It is what it is.", NewPronounInflectionBe(), 0)
}

func TestPronounInflectionBe_AllowsTheyAreNegation(t *testing.T) {
	linttest.AssertLintCount(t, "They are not interested.", NewPronounInflectionBe(), 0)
}

func TestPronounInflectionBe_AllowsTheyWere(t *testing.T) {
	linttest.AssertLintCount(t, "They were already here.", NewPronounInflectionBe(), 0)
}

func TestPronounInflectionBe_AllowsAsdfIs(t *testing.T) {
	linttest.AssertLintCount(t, "asdf is not a word", NewPronounInflectionBe(), 0)
}

func TestPronounInflectionBe_NoSubject(t *testing.T) {
	linttest.AssertLintCount(t, "is set", NewPronounInflectionBe(), 0)
}

func TestPronounInflectionBe_CorrectsIWere(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I were the best player on the field.", NewPronounInflectionBe(), "I was the best player on the field.")
}

func TestPronounInflectionBe_CorrectsWeWas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We was the best players on the field.", NewPronounInflectionBe(), "We were the best players on the field.")
}

func TestPronounInflectionBe_CorrectsYouWas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You was my best friend.", NewPronounInflectionBe(), "You were my best friend.")
}

func TestPronounInflectionBe_AllowsYouWere(t *testing.T) {
	linttest.AssertLintCount(t, "You were my best friend.", NewPronounInflectionBe(), 0)
}

func TestPronounInflectionBe_CorrectsHeWere(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He were late.", NewPronounInflectionBe(), "He was late.")
}

func TestPronounInflectionBe_CorrectsTheyWas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They was on time.", NewPronounInflectionBe(), "They were on time.")
}

func TestPronounInflectionBe_AllowsHeWas(t *testing.T) {
	linttest.AssertLintCount(t, "He was here.", NewPronounInflectionBe(), 0)
}

func TestPronounInflectionBe_AllowsWeWere(t *testing.T) {
	linttest.AssertLintCount(t, "We were excited.", NewPronounInflectionBe(), 0)
}

func TestPronounInflectionBe_CorrectsHeArent(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He aren't ready.", NewPronounInflectionBe(), "He isn't ready.")
}

func TestPronounInflectionBe_CorrectsTheyIsnt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They isn't coming.", NewPronounInflectionBe(), "They aren't coming.")
}

func TestPronounInflectionBe_AllowsHeIsnt(t *testing.T) {
	linttest.AssertLintCount(t, "He isn't ready.", NewPronounInflectionBe(), 0)
}

func TestPronounInflectionBe_AllowsTheyArent(t *testing.T) {
	linttest.AssertLintCount(t, "They aren't coming.", NewPronounInflectionBe(), 0)
}

func TestPronounInflectionBe_CorrectsSheReallyAre(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She really are talented.", NewPronounInflectionBe(), "She really is talented.")
}

func TestPronounInflectionBe_CorrectsTheyOftenIs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They often is late.", NewPronounInflectionBe(), "They often are late.")
}

func TestPronounInflectionBe_CorrectsBecauseHeAre(t *testing.T) {
	linttest.AssertSuggestionResult(t, "because he are tired.", NewPronounInflectionBe(), "because he is tired.")
}

func TestPronounInflectionBe_AllowBehindHim(t *testing.T) {
	linttest.AssertNoLints(t, "Behind him are new shadows.", NewPronounInflectionBe())
}

func TestPronounInflectionBe_Issue1682(t *testing.T) {
	linttest.AssertNoLints(t, "Understanding them is significant", NewPronounInflectionBe())
}
