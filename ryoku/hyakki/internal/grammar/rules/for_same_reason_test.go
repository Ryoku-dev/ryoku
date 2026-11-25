package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestForSameReason_ForFinancialReasons(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... tries to hide the real cause of death or for financial reason, ...", NewForSameReason(), "... tries to hide the real cause of death or for financial reasons, ...")
}

func TestForSameReason_DontFlagForHumanReason(t *testing.T) {
	linttest.AssertNoLints(t, "the platonic ideal of reasoning is not even an approximation for human reason", NewForSameReason())
}

func TestForSameReason_ForObviousReasons(t *testing.T) {
	linttest.AssertSuggestionResult(t, "this will add 1-bit per cycle, but no body uses this for obvious reason, it's just a toy example ...", NewForSameReason(), "this will add 1-bit per cycle, but no body uses this for obvious reasons, it's just a toy example ...")
}

func TestForSameReason_ForObviousReasons2(t *testing.T) {
	linttest.AssertSuggestionResult(t, "it is certainly not the same system, for obvious reason", NewForSameReason(), "it is certainly not the same system, for obvious reasons")
}

func TestForSameReason_ForOtherReasons(t *testing.T) {
	linttest.AssertSuggestionResult(t, "sometimes as I touched them for other reason and other times as independent changes", NewForSameReason(), "sometimes as I touched them for other reasons and other times as independent changes")
}

func TestForSameReason_ForOtherReasons2(t *testing.T) {
	linttest.AssertSuggestionResult(t, "but it might be greyed out for other reason though", NewForSameReason(), "but it might be greyed out for other reasons though")
}

func TestForSameReason_ForTheSameReason(t *testing.T) {
	linttest.AssertSuggestionResult(t, "we tried and failed for same reason", NewForSameReason(), "we tried and failed for the same reason")
}

func TestForSameReason_ForTheSameReason2(t *testing.T) {
	linttest.AssertSuggestionResult(t, "For same reason AI controled surgery isn't going to replace surgical school experience", NewForSameReason(), "For the same reason AI controled surgery isn't going to replace surgical school experience")
}

func TestForSameReason_ForSecurityReasons(t *testing.T) {
	linttest.AssertSuggestionResult(t, "HTTPS is required for security reason", NewForSameReason(), "HTTPS is required for security reasons")
}

func TestForSameReason_ForSimilarReasons(t *testing.T) {
	linttest.AssertSuggestionResult(t, "For similar reason you can't use Earth's magnetic field", NewForSameReason(), "For similar reasons you can't use Earth's magnetic field")
}

func TestForSameReason_ForSimilarReasons2(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is, in some sense, for similar reason to Gunnar Carlson et al finding a Klein bottle", NewForSameReason(), "This is, in some sense, for similar reasons to Gunnar Carlson et al finding a Klein bottle")
}

func TestForSameReason_ForVariousReasons(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We decided for various reason to move back to Finland", NewForSameReason(), "We decided for various reasons to move back to Finland")
}
