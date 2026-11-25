package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestHandfulOfMore_FixHandfulOfMoreTimes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "(and possibly a handful of more times as needed to resolve any issues)", NewHandfulOfMore(), "(and possibly a handful more times as needed to resolve any issues)")
}

func TestHandfulOfMore_FixHandfulOfMorePrs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "(maybe as soon as tomorrow, but trying to get a handful of more PRs merged...)", NewHandfulOfMore(), "(maybe as soon as tomorrow, but trying to get a handful more PRs merged...)")
}

func TestHandfulOfMore_DontFlagComparative(t *testing.T) {
	linttest.AssertNoLints(t, "After writing a handful of more advanced recipes I often found myself running into situations where the context wasn't recognized", NewHandfulOfMore())
}
