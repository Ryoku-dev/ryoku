package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestExceptOf_FixTheExceptOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "With the except of one particular sender, all the other senders are sending correctly to the configured smarthost.", NewExceptOf(), "With the exception of one particular sender, all the other senders are sending correctly to the configured smarthost.")
}

func TestExceptOf_FixPossibleExceptOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "With the possible except of two vertexes with one having one more in-degree than out-degree", NewExceptOf(), "With the possible exception of two vertexes with one having one more in-degree than out-degree")
}

func TestExceptOf_FixNotableExceptOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "identical at the end of the day to never logging into it again (with the notable except of potentially identifying info you've left in your profile, of course)", NewExceptOf(), "identical at the end of the day to never logging into it again (with the notable exception of potentially identifying info you've left in your profile, of course)")
}

func TestExceptOf_IgnoreExceptOfCourse(t *testing.T) {
	linttest.AssertNoLints(t, "much like the existing config option access_log_format, except of course for the error log (option errorlog) output format.", NewExceptOf())
}

func TestExceptOf_FixExceptOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "There are no usable way to restart worker except of supervisorctl restart workername", NewExceptOf(), "There are no usable way to restart worker except for supervisorctl restart workername")
}
