package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestGetAccessAt_TestGetAccessAtCpu(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They really need to get access at some CPU time.", NewGetAccessAt(), "They really need to get access to some CPU time.")
}

func TestGetAccessAt_TestGetAccessAtModels(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In order to get access at models, send a request.", NewGetAccessAt(), "In order to get access to models, send a request.")
}

func TestGetAccessAt_TestGotAccessAtDatabase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I got access at the server database.", NewGetAccessAt(), "I got access to the server database.")
}

func TestGetAccessAt_IgnoreAccessAtAll(t *testing.T) {
	linttest.AssertLintCount(t, "I didn't manage to get access at all.", NewGetAccessAt(), 0)
}

func TestGetAccessAt_IgnoreAccessAtLeast(t *testing.T) {
	linttest.AssertLintCount(t, "Is it possible to get access at least to the file content?", NewGetAccessAt(), 0)
}

func TestGetAccessAt_IgnoreAccessAtThisTime(t *testing.T) {
	linttest.AssertLintCount(t, "The quickest method to get access at this time is to submit a form.", NewGetAccessAt(), 0)
}

func TestGetAccessAt_IgnoreAccessAtUrl(t *testing.T) {
	linttest.AssertLintCount(t, "Get access at https://example.com/login", NewGetAccessAt(), 0)
}
