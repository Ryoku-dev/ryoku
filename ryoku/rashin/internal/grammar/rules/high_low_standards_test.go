package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestHighLowStandards_AtEnd(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have low standard", NewHighLowStandards(), "I have low standards")
}

func TestHighLowStandards_FollowedByFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have high standard for men", NewHighLowStandards(), "I have high standards for men")
}

func TestHighLowStandards_DontFlagStandardDeviation(t *testing.T) {
	linttest.AssertNoLints(t, "It's very difficult for any model to predict accurately when trained with data having such high standard deviation.", NewHighLowStandards())
}

func TestHighLowStandards_DontFlagFollowedByWhen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I tend to have high standard when answering questions.", NewHighLowStandards(), "I tend to have high standards when answering questions.")
}

func TestHighLowStandards_FollowedByComma(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Most student would choose supervisor who doesn’t have high standard, but what i did is precisely the opposite.", NewHighLowStandards(), "Most student would choose supervisor who doesn’t have high standards, but what i did is precisely the opposite.")
}

func TestHighLowStandards_VeryHigh(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I picked a supervisor who has very high standard, strict, and very well detailed.", NewHighLowStandards(), "I picked a supervisor who has very high standards, strict, and very well detailed.")
}

func TestHighLowStandards_FollowedByAnd(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Great way to tell you have low standard and can't detect latency.", NewHighLowStandards(), "Great way to tell you have low standards and can't detect latency.")
}

func TestHighLowStandards_DontFlagStandardOfLiving(t *testing.T) {
	linttest.AssertNoLints(t, "US has high standard of living, but it is not among the top 5 countreis with the highest minimum hourly-wage.", NewHighLowStandards())
}

func TestHighLowStandards_FollowedByOn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We should also recognize the fact that, Swedes have very high standard on their expectation of English fluency.", NewHighLowStandards(), "We should also recognize the fact that, Swedes have very high standards on their expectation of English fluency.")
}

func TestHighLowStandards_FollowedByIn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We have high standard in product manufacturing and foreign trade", NewHighLowStandards(), "We have high standards in product manufacturing and foreign trade")
}
