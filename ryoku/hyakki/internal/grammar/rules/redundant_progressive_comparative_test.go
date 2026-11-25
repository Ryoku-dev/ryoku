package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestRedundantProgressiveComparative_FixesIncreasinglyMore(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The issue is increasingly more prevalent in distributed systems.", NewRedundantProgressiveComparative(), "The issue is more and more prevalent in distributed systems.")
}

func TestRedundantProgressiveComparative_FixesIncreasinglyLess(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The outages are increasingly less frequent after the migration.", NewRedundantProgressiveComparative(), "The outages are less and less frequent after the migration.")
}

func TestRedundantProgressiveComparative_PreservesMatchCaseTitle(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The bug is Increasingly More Prevalent in nightly builds.", NewRedundantProgressiveComparative(), "The bug is More and more Prevalent in nightly builds.")
}

func TestRedundantProgressiveComparative_PreservesMatchCaseAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The bug is INCREASINGLY MORE PREVALENT in nightly builds.", NewRedundantProgressiveComparative(), "The bug is MORE AND MORE PREVALENT in nightly builds.")
}

func TestRedundantProgressiveComparative_PreservesMatchCaseLessAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The regressions are INCREASINGLY LESS SEVERE with each patch.", NewRedundantProgressiveComparative(), "The regressions are LESS AND LESS SEVERE with each patch.")
}

func TestRedundantProgressiveComparative_IgnoresProgressivelyMore(t *testing.T) {
	linttest.AssertNoLints(t, "These failures are progressively more severe in production.", NewRedundantProgressiveComparative())
}

func TestRedundantProgressiveComparative_IgnoresSteadilyMore(t *testing.T) {
	linttest.AssertNoLints(t, "The interface is steadily more accessible each release.", NewRedundantProgressiveComparative())
}

func TestRedundantProgressiveComparative_IgnoresProgressivelyLess(t *testing.T) {
	linttest.AssertNoLints(t, "The warnings became progressively less noticeable over time.", NewRedundantProgressiveComparative())
}

func TestRedundantProgressiveComparative_IgnoresSteadilyLess(t *testing.T) {
	linttest.AssertNoLints(t, "The logs are steadily less noisy in recent builds.", NewRedundantProgressiveComparative())
}

func TestRedundantProgressiveComparative_IgnoresMoreThan(t *testing.T) {
	linttest.AssertNoLints(t, "The issue is increasingly more than a minor annoyance.", NewRedundantProgressiveComparative())
}

func TestRedundantProgressiveComparative_IgnoresLessThan(t *testing.T) {
	linttest.AssertNoLints(t, "The issue is increasingly less than a minor annoyance.", NewRedundantProgressiveComparative())
}

func TestRedundantProgressiveComparative_IgnoresNounAfterMore(t *testing.T) {
	linttest.AssertNoLints(t, "The issue is increasingly more people reporting crashes.", NewRedundantProgressiveComparative())
}

func TestRedundantProgressiveComparative_IgnoresNounAfterLess(t *testing.T) {
	linttest.AssertNoLints(t, "The issue is increasingly less people reporting crashes.", NewRedundantProgressiveComparative())
}

func TestRedundantProgressiveComparative_IgnoresWithoutComparator(t *testing.T) {
	linttest.AssertNoLints(t, "The issue is increasingly prevalent in this subsystem.", NewRedundantProgressiveComparative())
}

func TestRedundantProgressiveComparative_IgnoresOtherAdverbialComparatives(t *testing.T) {
	linttest.AssertNoLints(t, "The issue is much more prevalent in this subsystem.", NewRedundantProgressiveComparative())
	linttest.AssertNoLints(t, "The issue is significantly less prevalent in this subsystem.", NewRedundantProgressiveComparative())
	linttest.AssertNoLints(t, "The issue is ever more prevalent in this subsystem.", NewRedundantProgressiveComparative())
}

func TestRedundantProgressiveComparative_MessageMentionsLessAndLessForLessBranch(t *testing.T) {
	linttest.AssertLintMessage(t, "The trend is increasingly less stable over time.", NewRedundantProgressiveComparative(), "This phrasing is redundant; use a direct comparative like `less and less`.")
}

func TestRedundantProgressiveComparative_EmitsExpectedLintCount(t *testing.T) {
	linttest.AssertLintCount(t, "The trend is increasingly more likely in larger teams.", NewRedundantProgressiveComparative(), 1)
}
