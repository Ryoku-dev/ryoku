package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestRepeatedWords_CatchesBasic(t *testing.T) {
	linttest.AssertLintCount(t, "I wanted the the banana.", NewRepeatedWords(), 1)
}

func TestRepeatedWords_DoesNotLintHomographsAddress(t *testing.T) {
	linttest.AssertLintCount(t, "To address address problems.", NewRepeatedWords(), 0)
}

func TestRepeatedWords_DoesNotLintHomographsRecord(t *testing.T) {
	linttest.AssertLintCount(t, "To record record profits.", NewRepeatedWords(), 0)
}

func TestRepeatedWords_Issue253(t *testing.T) {
	linttest.AssertLintCount(t, "this paper shows that, while the method may be more accurate accurate, the turnout overestimate suggests that self-selection bias is not sufficiently reduced", NewRepeatedWords(), 1)
}

func TestRepeatedWords_Issue333(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is is a test", NewRepeatedWords(), "This is a test")
}

func TestRepeatedWords_DoubleA(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is a a test", NewRepeatedWords(), "This is a test")
}

func TestRepeatedWords_DoubleAnd(t *testing.T) {
	linttest.AssertSuggestionResult(t, "And and this is also a test", NewRepeatedWords(), "And this is also a test")
}

func TestRepeatedWords_OnOnGithub(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Take a look at the project on on GitHub.", NewRepeatedWords(), "Take a look at the project on GitHub.")
}

func TestRepeatedWords_AsAs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "he is as as hard as nails", NewRepeatedWords(), "he is as hard as nails")
}

func TestRepeatedWords_DontFlagFirstHyphenated(t *testing.T) {
	linttest.AssertLintCount(t, "The driver-facing camera and microphone are only logged if you explicitly opt-in in settings.", NewRepeatedWords(), 0)
}

func TestRepeatedWords_DontFlagHyphenatedEitherSide(t *testing.T) {
	linttest.AssertLintCount(t, "foo-foo foo bar bar-bar", NewRepeatedWords(), 0)
}

func TestRepeatedWords_DontFlagRepeatedLaughter(t *testing.T) {
	for _, source := range []string{"ha ha", "Ha ha", "hah hah", "ha ha ha"} {
		linttest.AssertLintCount(t, source, NewRepeatedWords(), 0)
	}
}
