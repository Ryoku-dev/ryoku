package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestNoLongerPronoun_FixNoLongerYouNeedTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It works great for poetry and no longer you need to create keys on pypi", NewNoLongerPronoun(), "It works great for poetry and you no longer need to create keys on pypi")
}

func TestNoLongerPronoun_FixNoLongerIt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "does that mean that i have make myself an intranet library and no longer it automatically creates a library with my username?", NewNoLongerPronoun(), "does that mean that i have make myself an intranet library and it no longer automatically creates a library with my username?")
}

func TestNoLongerPronoun_FixNoLongerIAmAbleVerify(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the hash value got replaced by the app's name and no longer I am able to verify OTP automatically", NewNoLongerPronoun(), "the hash value got replaced by the app's name and I am no longer able to verify OTP automatically")
}

func TestNoLongerPronoun_FixNoLongerIAmAbleSee(t *testing.T) {
	linttest.AssertSuggestionResult(t, "there is also 'effect' to be applied, but no longer I am able to see and control their colors", NewNoLongerPronoun(), "there is also 'effect' to be applied, but I am no longer able to see and control their colors")
}

func TestNoLongerPronoun_NoLongerWeAreInterested(t *testing.T) {
	t.Skip("I think the word 'even' being in the wrong place here is a separate mistake")
	linttest.AssertSuggestionResult(t, "I do not also care about what you think and even no longer we are interested to demo tamota buggy feature for our class", NewNoLongerPronoun(), "I do not also care about what you think and we are no longer even interested to demo tamota buggy feature for our class")
}

func TestNoLongerPronoun_AllowIWho(t *testing.T) {
	linttest.AssertNoLints(t, "It is no longer I who live, but Christ who lives in me", NewNoLongerPronoun())
}

func TestNoLongerPronoun_DifferentMistakeMissingWord(t *testing.T) {
	t.Skip("If this turns out to be common, checking for a modal verb before or 'to' after might work")
	linttest.AssertNoLints(t, "If I understand this change correctly this change would no longer you to duplicate materials or make them unique to pass in unique parameters", NewNoLongerPronoun())
}
