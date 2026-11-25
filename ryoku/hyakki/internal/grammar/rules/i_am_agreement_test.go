package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestIAmAgreement_CorrectsIAreSimple(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I are", NewIAmAgreement(), "I am")
}

func TestIAmAgreement_CorrectsIAre(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I are really happy about this release.", NewIAmAgreement(), "I am really happy about this release.")
}

func TestIAmAgreement_DontFlagYouAndIAre(t *testing.T) {
	linttest.AssertLintCount(t, "You know, you and I are sitting on the Titanic and the iceberg is over there.", NewIAmAgreement(), 0)
}

func TestIAmAgreement_DontFlagMentionAndIAre(t *testing.T) {
	linttest.AssertLintCount(t, "Hello, @another-rex and I are attempting to package packageurl-go for Debian as we need it for a build dependency.", NewIAmAgreement(), 0)
}

func TestIAmAgreement_DontFlagZAndIAre(t *testing.T) {
	linttest.AssertLintCount(t, "The url is copied from a manual search, and Z and I are modified.", NewIAmAgreement(), 0)
}

func TestIAmAgreement_DontFlagNameAndIAre(t *testing.T) {
	linttest.AssertLintCount(t, "Paper that Lena Baunaz and I are working on as part of my SNSF-funded 'Focus in diachrony'", NewIAmAgreement(), 0)
}

func TestIAmAgreement_FixSoIAre(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have not yet been able to reproduce this issue in my environment, so I are still trying to figure it out", NewIAmAgreement(), "I have not yet been able to reproduce this issue in my environment, so I am still trying to figure it out")
}

func TestIAmAgreement_FixIfIAre(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If i are on creative inventory, and try to clean my inventory holding shift is disconnected too.", NewIAmAgreement(), "If i am on creative inventory, and try to clean my inventory holding shift is disconnected too.")
}

func TestIAmAgreement_FixWhatIAre(t *testing.T) {
	linttest.AssertSuggestionResult(t, "in this situation I can't see what I are typing", NewIAmAgreement(), "in this situation I can't see what I am typing")
}

func TestIAmAgreement_FixWhereIAre(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have a logging application where I are append to a topic", NewIAmAgreement(), "I have a logging application where I am append to a topic")
}
