package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAsHow_Advise(t *testing.T) {
	linttest.AssertSuggestionResult(t, "[BUG] Advise as how to debug error messages.", NewAsHow(), "[BUG] Advise as to how to debug error messages.")
}

func TestAsHow_Puzzled(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Since I'm using OpenAPI to describe my API I'm puzzled as how to describe this response.", NewAsHow(), "Since I'm using OpenAPI to describe my API I'm puzzled as to how to describe this response.")
}

func TestAsHow_Example(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A sample/example as how to use Summaries", NewAsHow(), "A sample/example as to how to use Summaries")
}

func TestAsHow_Clueless(t *testing.T) {
	linttest.AssertSuggestionResult(t, "searched the issues/questions but I'm still clueless as how to set the body of the HTTP request", NewAsHow(), "searched the issues/questions but I'm still clueless as to how to set the body of the HTTP request")
}

func TestAsHow_Examples(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We would need a more concrete specification together with examples as how this would be supposed to work.", NewAsHow(), "We would need a more concrete specification together with examples as to how this would be supposed to work.")
}

func TestAsHow_Confused(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hi! I'm confused as how to save the brain bytes model", NewAsHow(), "Hi! I'm confused as to how to save the brain bytes model")
}

func TestAsHow_Tackle(t *testing.T) {
	linttest.AssertSuggestionResult(t, "it's more of a design document/me laying my thoughts out as how to tackle this problem", NewAsHow(), "it's more of a design document/me laying my thoughts out as to how to tackle this problem")
}

func TestAsHow_AvoidAsWellAsHow(t *testing.T) {
	linttest.AssertNoLints(t, "When to do Git things (as well as how)", NewAsHow())
}

func TestAsHow_AvoidAsWellAsThey(t *testing.T) {
	linttest.AssertNoLints(t, "Some questions on configuring capsules, as well as how they are (or are not) weighted.", NewAsHow())
}

func TestAsHow_AvoidAsWellAsHowTo(t *testing.T) {
	linttest.AssertNoLints(t, "how to contribute a feature to PolyScope as well as how to modify and remove it", NewAsHow())
}

func TestAsHow_AvoidAsMuchAs(t *testing.T) {
	linttest.AssertNoLints(t, "It recognizes that how code looks matters just as much as how it works.", NewAsHow())
}

func TestAsHow_AvoidAsFarAs(t *testing.T) {
	linttest.AssertNoLints(t, "Although the renderer will be opinionated as far as how it will structure the submission data out-of-the-box", NewAsHow())
}

func TestAsHow_AvoidTheSameAs(t *testing.T) {
	linttest.AssertNoLints(t, "FILE_ALL_ACCESS is not defined the same as how Windows defines it", NewAsHow())
}

func TestAsHow_HardToParse(t *testing.T) {
	t.Skip("we probably can't handle this case no matter what")
	linttest.AssertNoLints(t, "As how many requests is a request with a list of coordinates counted?", NewAsHow())
}
