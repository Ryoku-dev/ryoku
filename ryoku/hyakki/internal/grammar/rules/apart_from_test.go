package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestApartFrom_CorrectsBasicTypo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Christianity was set apart form other religions.", NewApartFrom(), "Christianity was set apart from other religions.")
}

func TestApartFrom_CorrectsTitleCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Apart Form these files, everything uploaded fine.", NewApartFrom(), "Apart From these files, everything uploaded fine.")
}

func TestApartFrom_CorrectsAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "APART FORM THE REST OF THE FIELD.", NewApartFrom(), "APART FROM THE REST OF THE FIELD.")
}

func TestApartFrom_CorrectsWithComma(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It was apart form, not apart from, the original plan.", NewApartFrom(), "It was apart from, not apart from, the original plan.")
}

func TestApartFrom_CorrectsWithNewline(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They stood apart\nform everyone else at the rally.", NewApartFrom(), "They stood apart\nfrom everyone else at the rally.")
}

func TestApartFrom_CorrectsExtraSpacing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We keep the archive apart   form public assets.", NewApartFrom(), "We keep the archive apart   from public assets.")
}

func TestApartFrom_AllowsCorrectPhrase(t *testing.T) {
	linttest.AssertLintCount(t, "Lebanon's freedoms set it apart from other Arab states.", NewApartFrom(), 0)
}

func TestApartFrom_IgnoresHyphenated(t *testing.T) {
	linttest.AssertLintCount(t, "Their apart-form design wasn’t what we needed.", NewApartFrom(), 0)
}

func TestApartFrom_IgnoresSplitByComma(t *testing.T) {
	linttest.AssertLintCount(t, "They stood apart, form lines when asked.", NewApartFrom(), 0)
}

func TestApartFrom_IgnoresUnrelatedFormUsage(t *testing.T) {
	linttest.AssertLintCount(t, "The form was kept apart to dry after printing.", NewApartFrom(), 0)
}
