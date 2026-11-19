package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestOfCourse_FlagsOfCurse(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Yes, of curse!", NewOfCourse(), "Yes, of course!")
}

func TestOfCourse_FlagsOfCorse(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Well, of corse we can.", NewOfCourse(), "Well, of course we can.")
}

func TestOfCourse_IgnoresKindOfCurse(t *testing.T) {
	linttest.AssertLintCount(t, "This kind of curse is dangerous.", NewOfCourse(), 0)
}

func TestOfCourse_IgnoresSortOfCurse(t *testing.T) {
	linttest.AssertLintCount(t, "It's a sort of curse that lingers.", NewOfCourse(), 0)
}

func TestOfCourse_IgnoresCurseOfTitle(t *testing.T) {
	linttest.AssertLintCount(t, "The Curse of Strahd is a famous module.", NewOfCourse(), 0)
}

func TestOfCourse_FlagsOffCourse(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Yes, off course we should do that.", NewOfCourse(), "Yes, of course we should do that.")
}

func TestOfCourse_FlagsOCourse(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Yes, o course we should do that.", NewOfCourse(), "Yes, of course we should do that.")
}

func TestOfCourse_FlagsOfcourse(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Ofcourse, I like other languages.", NewOfCourse(), "Of course, I like other languages.")
}

func TestOfCourse_FlagsOffCoarse(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Off coarse, the web service will still be operational.", NewOfCourse(), "Of course, the web service will still be operational.")
}

func TestOfCourse_IgnoresLiteralOffCourse(t *testing.T) {
	linttest.AssertLintCount(t, "Her sailboat had been driven off course by the storm.", NewOfCourse(), 0)
}
