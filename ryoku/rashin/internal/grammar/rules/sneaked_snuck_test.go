package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestSneakedSnuck_CorrectSneakedToSnuck(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He sneaked in around the back.", NewPreferSnuck(), "He snuck in around the back.")
}

func TestSneakedSnuck_CorrectSneakedUppercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He Sneaked in around the back.", NewPreferSnuck(), "He Snuck in around the back.")
}

func TestSneakedSnuck_CorrectSneakedAllcaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He SNEAKED in around the back.", NewPreferSnuck(), "He SNUCK in around the back.")
}

func TestSneakedSnuck_CorrectToSnuckFromGithub(t *testing.T) {
	linttest.AssertSuggestionResult(t, "recycled transitions, lingering inflation, copula swaps that sneaked through", NewPreferSnuck(), "recycled transitions, lingering inflation, copula swaps that snuck through")
}

func TestSneakedSnuck_DontFlagSnuckWhenItIsPreferred(t *testing.T) {
	linttest.AssertNoLints(t, "I'm not sure exactly when this fix snuck in", NewPreferSnuck())
}

func TestSneakedSnuck_CorrectSnuckToSneaked(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He snuck in around the back.", NewPreferSneaked(), "He sneaked in around the back.")
}

func TestSneakedSnuck_CorrectSnuckUppercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He Snuck in around the back.", NewPreferSneaked(), "He Sneaked in around the back.")
}

func TestSneakedSnuck_CorrectSnuckAllcaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He SNUCK in around the back.", NewPreferSneaked(), "He SNEAKED in around the back.")
}

func TestSneakedSnuck_CorrectToSneakedFromGithub(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A few unhandled Errors snuck their way into the code base over time f.ex. here:", NewPreferSneaked(), "A few unhandled Errors sneaked their way into the code base over time f.ex. here:")
}

func TestSneakedSnuck_DontFlagSneakedWhenItIsPreferred(t *testing.T) {
	linttest.AssertNoLints(t, "Something related to recent experiments of WASM support sneaked into the main branch.", NewPreferSneaked())
}
