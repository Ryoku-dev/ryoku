package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestUnclosedQuotes_AllowsDialogueWithEmDashInterruption(t *testing.T) {
	linttest.AssertNoLints(t, "\"It'll be our\"—she leaned to his ear—\"shared secret.\"", NewUnclosedQuotes())
}

func TestUnclosedQuotes_StillFlagsUnclosedQuotes(t *testing.T) {
	linttest.AssertLintCount(t, "\"It'll be our", NewUnclosedQuotes(), 1)
}
