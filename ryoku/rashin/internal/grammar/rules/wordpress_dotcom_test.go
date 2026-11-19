package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestWordpressDotcom_Simple(t *testing.T) {
	linttest.AssertSuggestionResult(t, "wordpress.com", NewWordPressDotcom(), "WordPress.com")
}

func TestWordpressDotcom_Sentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "wordpress.com is a great hosting provider", NewWordPressDotcom(), "WordPress.com is a great hosting provider")
}
