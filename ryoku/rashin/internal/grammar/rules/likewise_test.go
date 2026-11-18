package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestLikewise_WiseMen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Like wise men, we waited.", NewLikewise(), "Like wise men, we waited.")
}

func TestLikewise_LikeWise(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He acted, like wise, without hesitation.", NewLikewise(), "He acted, likewise, without hesitation.")
}
