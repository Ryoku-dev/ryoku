package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestLongTimeAgo_FixLongTimeAgoMidSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Simple URL redirect handler written long time ago in PHP - yaroslav-ilin/shortener.", NewLongTimeAgo(), "Simple URL redirect handler written a long time ago in PHP - yaroslav-ilin/shortener.")
}

func TestLongTimeAgo_FixLongTimeAgoStartOfSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Long time ago, I wrote this code.", NewLongTimeAgo(), "A long time ago, I wrote this code.")
}

func TestLongTimeAgo_DontFlagALongTimeAgoMidSentence(t *testing.T) {
	linttest.AssertNoLints(t, "Created a long time ago and now finally saved in this gist as a backup.", NewLongTimeAgo())
}

func TestLongTimeAgo_DontFlagALongTimeAgoStartOfSentence(t *testing.T) {
	linttest.AssertNoLints(t, "a long time ago i had random site redirector on my we site that would send visitors to... - sites.txt.", NewLongTimeAgo())
}
