package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestRedundantAdditiveAdverbs_FlagAsWellToo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Yeah, we definitely miss him on this episode here, but you could probably get him on a podcast that's more focused on what Equinix is doing as well too, specifically.", NewRedundantAdditiveAdverbs(), "Yeah, we definitely miss him on this episode here, but you could probably get him on a podcast that's more focused on what Equinix is doing as well, specifically.")
}

func TestRedundantAdditiveAdverbs_FlagTooAlso(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The #1 uptime service with many servers and is easy to setup. It is free too also.", NewRedundantAdditiveAdverbs(), "The #1 uptime service with many servers and is easy to setup. It is free also.")
}

func TestRedundantAdditiveAdverbs_DontFlagAlsoToo(t *testing.T) {
	linttest.AssertLintCount(t, "The version update is also too slow.", NewRedundantAdditiveAdverbs(), 0)
}

func TestRedundantAdditiveAdverbs_DontFlagAlsoAsWellAs(t *testing.T) {
	linttest.AssertLintCount(t, "Believe there are stable packages in the readme also as well as a link to an old version of forge in the ...", NewRedundantAdditiveAdverbs(), 0)
}

func TestRedundantAdditiveAdverbs_DoFlagTooAlsoAsWellAs(t *testing.T) {
	linttest.AssertLintCount(t, "What would happen with a sentence that included too also as well as?", NewRedundantAdditiveAdverbs(), 1)
}

func TestRedundantAdditiveAdverbs_FlagTooAsWell(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Module name itself was changed too as well.", NewRedundantAdditiveAdverbs(), "Module name itself was changed as well.")
}
