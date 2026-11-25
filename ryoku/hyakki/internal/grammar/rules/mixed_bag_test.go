package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestMixedBag_AMixedBad(t *testing.T) {
	linttest.AssertSuggestionResult(t, "CommandLine interface is already a mixed bad of wstring and #ifdef to   string or wstring.", NewMixedBag(), "CommandLine interface is already a mixed bag of wstring and #ifdef to   string or wstring.")
}

func TestMixedBag_BigMixedBag(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Speaking of dungeons , the dungeons in this game are a big mixed bad.", NewMixedBag(), "Speaking of dungeons , the dungeons in this game are a big mixed bag.")
}

func TestMixedBag_DamnMixedBag(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is a damn mixed bad which left me frustrated, and yet longing for more.", NewMixedBag(), "This is a damn mixed bag which left me frustrated, and yet longing for more.")
}

func TestMixedBag_HugeMixedBag(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Also a huge mixed bad of no name monitors of different sizes that all have different color settings on.", NewMixedBag(), "Also a huge mixed bag of no name monitors of different sizes that all have different color settings on.")
}

func TestMixedBag_LargeMixedBag(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I’m still struggling to comprehend how it throws such a large mixed bad of symptoms in the mix this time.", NewMixedBag(), "I’m still struggling to comprehend how it throws such a large mixed bag of symptoms in the mix this time.")
}

func TestMixedBag_MassiveMixedBad(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Anyway. In topic, Swano was a massive mixed bad in this game.", NewMixedBag(), "Anyway. In topic, Swano was a massive mixed bag in this game.")
}

func TestMixedBag_MassivelyMixedBag(t *testing.T) {
	linttest.AssertSuggestionResult(t, "While certain things are more common to be either way, it's a massively mixed bad overall.", NewMixedBag(), "While certain things are more common to be either way, it's a massively mixed bag overall.")
}

func TestMixedBag_PrettyMixedBag(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's a pretty mixed bad for me: Evolution Xavier for comic Xavier. Evolution Magneto for comic Magneto.", NewMixedBag(), "It's a pretty mixed bag for me: Evolution Xavier for comic Xavier. Evolution Magneto for comic Magneto.")
}

func TestMixedBag_RatherMixedBag(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Well chaps, as expected the TS contains a rather mixed bad of promise and disappointment.", NewMixedBag(), "Well chaps, as expected the TS contains a rather mixed bag of promise and disappointment.")
}

func TestMixedBag_ReallyMixedBag(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is a really mixed bad On one hand you have some of Eminem's highest highs and his lowest lows but ever.", NewMixedBag(), "This is a really mixed bag On one hand you have some of Eminem's highest highs and his lowest lows but ever.")
}

func TestMixedBag_SlightlyMixedBag(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I absolutely love Yes Minister and Yes Prime Minister but it did end up a slightly mixed bad in terms of impact.", NewMixedBag(), "I absolutely love Yes Minister and Yes Prime Minister but it did end up a slightly mixed bag in terms of impact.")
}

func TestMixedBag_SomewhatMixedBag(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A somewhat mixed bad. The space is pleasant with a rustic vibe.", NewMixedBag(), "A somewhat mixed bag. The space is pleasant with a rustic vibe.")
}

func TestMixedBag_VeryMixedBag(t *testing.T) {
	linttest.AssertSuggestionResult(t, "AVAILABLE MEN is a very mixed bad of short films about gay subjects that very from excellent to weak.", NewMixedBag(), "AVAILABLE MEN is a very mixed bag of short films about gay subjects that very from excellent to weak.")
}
