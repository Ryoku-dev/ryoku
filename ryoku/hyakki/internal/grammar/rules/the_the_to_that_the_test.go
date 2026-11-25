package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestTheTheToThatThe_DontFlagSimpleTheThe(t *testing.T) {
	linttest.AssertNoLints(t, "this is the the simple case", NewTheTheToThatThe())
}

func TestTheTheToThatThe_FixSoThatThe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "this is so the the other case is tested", NewTheTheToThatThe(), "this is so that the other case is tested")
}

func TestTheTheToThatThe_FactTheTheToThat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This PR fixes the fact the the / page and some of the other new pages are reverting to the browser font", NewTheTheToThatThe(), "This PR fixes the fact that the / page and some of the other new pages are reverting to the browser font")
}

func TestTheTheToThatThe_FoundTheTheJustDouble(t *testing.T) {
	linttest.AssertSuggestionResult(t, "and I've never found the the dashboard feed materially useful", NewTheTheToThatThe(), "and I've never found the dashboard feed materially useful")
}

func TestTheTheToThatThe_FoundTheTheCouldBeEither(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I found the the shape of feature from the PillarFeatureNet becomes [64] instead of [N, 64]", NewTheTheToThatThe(), []string{
		"I found that the shape of feature from the PillarFeatureNet becomes [64] instead of [N, 64]",
		"I found the shape of feature from the PillarFeatureNet becomes [64] instead of [N, 64]",
	}, []string{})
}

func TestTheTheToThatThe_FoundTheTheToThat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I was directed to the MongoDB website and found the the newer versions require you to have the mongosh shell", NewTheTheToThatThe(), "I was directed to the MongoDB website and found that the newer versions require you to have the mongosh shell")
}

func TestTheTheToThatThe_SayTheTheToThat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So, I'm going to say the the shown output is a bug in using Avro", NewTheTheToThatThe(), "So, I'm going to say that the shown output is a bug in using Avro")
}

func TestTheTheToThatThe_ShowTheTheJustDouble(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It would be better to show the the first logical line (i.e. the first paragraph).", NewTheTheToThatThe(), "It would be better to show the first logical line (i.e. the first paragraph).")
}

func TestTheTheToThatThe_SoTheTheCouldBeEither(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "accordion-body doesn't have data- attributes to read from, so the The proper solution isn't quite working", NewTheTheToThatThe(), []string{

		"accordion-body doesn't have data- attributes to read from, so the proper solution isn't quite working",
	}, []string{})
}

func TestTheTheToThatThe_SoTheTheJustDouble(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So the the choice is really between pursuing a compromise ...", NewTheTheToThatThe(), "So the choice is really between pursuing a compromise ...")
}

func TestTheTheToThatThe_ThinkTheTheToThat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I don't think the the additional flask route was the issue because I got your example to work.", NewTheTheToThatThe(), "I don't think that the additional flask route was the issue because I got your example to work.")
}

func TestTheTheToThatThe_ThinkTheTheCouldBeEither(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "but I think the the lack of transparency and attribution to the original coder makes this much more an act of opportunism", NewTheTheToThatThe(), []string{
		"but I think that the lack of transparency and attribution to the original coder makes this much more an act of opportunism",
		"but I think the lack of transparency and attribution to the original coder makes this much more an act of opportunism",
	}, []string{})
}
