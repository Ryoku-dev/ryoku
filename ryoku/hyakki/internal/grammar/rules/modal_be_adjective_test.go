package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestModalBeAdjective_FixWouldNice(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It would nice if Harper could detect this.", NewModalBeAdjective(), "It would be nice if Harper could detect this.")
}

func TestModalBeAdjective_FixCouldConfigured(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It could configured by parameters and the commands above effectively disable it.", NewModalBeAdjective(), "It could be configured by parameters and the commands above effectively disable it.")
}

func TestModalBeAdjective_FixWillAccessible(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Your WordPress site will accessible at http://localhost", NewModalBeAdjective(), "Your WordPress site will be accessible at http://localhost")
}

func TestModalBeAdjective_IgnoreWouldExternalTraffic(t *testing.T) {
	linttest.AssertNoLints(t, "And why would external traffic be trying to access my server if I don't know who or what it is?", NewModalBeAdjective())
}

func TestModalBeAdjective_IgnoreCouldKindOf(t *testing.T) {
	linttest.AssertNoLints(t, "you could kind of see the ...", NewModalBeAdjective())
}

func TestModalBeAdjective_IgnoreYouCanBackup(t *testing.T) {
	t.Skip("Upstream ignored: false positive: 'backup' is an adjective but also a spello for the verb 'back up'")
	linttest.AssertNoLints(t, "You can backup Userdata.", NewModalBeAdjective())
}

func TestModalBeAdjective_IgnoreWouldIncorrect(t *testing.T) {
	t.Skip("Upstream ignored: false positive: 'incorrect' should be 'incorrectly'.")
	linttest.AssertNoLints(t, "Bug in versions 4.0 and 4.1 would incorrect list the address module", NewModalBeAdjective())
}

func TestModalBeAdjective_IgnoreShouldUpper(t *testing.T) {
	t.Skip("Upstream ignored: false positive: 'upper-bound' is an ad-hoc verb here.")
	linttest.AssertNoLints(t, "we should upper-bound it to the next MAJOR version.", NewModalBeAdjective())
	linttest.AssertNoLints(t, "some older software (filezilla on debian-stable) cannot passive-mode with TLS", NewModalBeAdjective())
}

func TestModalBeAdjective_IgnoreSoftFork3168(t *testing.T) {
	linttest.AssertNoLints(t, "You should soft-fork the repository.", NewModalBeAdjective())
}

func TestModalBeAdjective_IgnoreCantWay(t *testing.T) {
	linttest.AssertNoLints(t, "I can't way for something better to be released.", NewModalBeAdjective())
}

func TestModalBeAdjective_IgnoreWouldLater(t *testing.T) {
	linttest.AssertNoLints(t, "She would later become president.", NewModalBeAdjective())
}
