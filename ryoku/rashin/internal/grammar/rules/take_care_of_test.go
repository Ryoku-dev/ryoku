package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestTakeCareOf_FixTake(t *testing.T) {
	linttest.AssertSuggestionResult(t, "OSGi: Take care about dynamically changed routes (and other initial data)", NewTakeCareOf(), "OSGi: Take care of dynamically changed routes (and other initial data)")
}

func TestTakeCareOf_FixTaken(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But this would mean that the creator of the logfile has taken care about making the fraction 6 characters long", NewTakeCareOf(), "But this would mean that the creator of the logfile has taken care of making the fraction 6 characters long")
}

func TestTakeCareOf_FixTakes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I like that Dart actively takes care about this and provides this consistent package", NewTakeCareOf(), "I like that Dart actively takes care of this and provides this consistent package")
}

func TestTakeCareOf_FixTaking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "second one is taking care about analysis of output", NewTakeCareOf(), "second one is taking care of analysis of output")
}

func TestTakeCareOf_FixTook(t *testing.T) {
	linttest.AssertSuggestionResult(t, "then this * function took care about the ndisc option", NewTakeCareOf(), "then this * function took care of the ndisc option")
}
