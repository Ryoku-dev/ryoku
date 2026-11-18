package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestWillNonLemma_FixWillRan(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "The brown fox will ran thru the meadow.", NewWillNonLemma(dict.Curated()), []string{
		"The brown fox will run thru the meadow.",
		"The brown fox ran thru the meadow.",
	}, []string{})
}

func TestWillNonLemma_FixWillExists(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "there is a good chance duplicate Rule IDs will exists.", NewWillNonLemma(dict.Curated()), []string{
		"there is a good chance duplicate Rule IDs will exist.",
		"there is a good chance duplicate Rule IDs exists.",
		"there is a good chance duplicate Rule IDs exist.",
	}, []string{})
}

func TestWillNonLemma_IgnoreShallVessels(t *testing.T) {
	linttest.AssertLintCount(t, "No Preference shall be given by any Regulation of Commerce or Revenue to the Ports of one State over those of another; nor shall Vessels bound to, or from, one State, be obliged to enter, clear, or pay Duties in another.", NewWillNonLemma(dict.Curated()), 0)
}

func TestWillNonLemma_IgnoreWillTools(t *testing.T) {
	linttest.AssertLintCount(t, "Give your AI free will tools.", NewWillNonLemma(dict.Curated()), 0)
}

func TestWillNonLemma_FixWillComingSoon(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "More advanced features will coming soon, so stay tuned!", NewWillNonLemma(dict.Curated()), []string{
		"More advanced features will come soon, so stay tuned!",
		"More advanced features coming soon, so stay tuned!",
		"More advanced features will be coming soon, so stay tuned!",
	}, []string{})
}

func TestWillNonLemma_FixWillComingNext(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "on CPU and GPU (NPU support will coming next)", NewWillNonLemma(dict.Curated()), []string{
		"on CPU and GPU (NPU support will come next)",
		"on CPU and GPU (NPU support coming next)",
		"on CPU and GPU (NPU support will be coming next)",
	}, []string{})
}

func TestWillNonLemma_IgnoreWillWasReadPrReview(t *testing.T) {
	linttest.AssertLintCount(t, "Around November 2023, shortly after the will was read, Eleanor started asking about using the cottage.", NewWillNonLemma(dict.Curated()), 0)
}

func TestWillNonLemma_IgnoreWillWasStraightforwardPrReview(t *testing.T) {
	linttest.AssertLintCount(t, "Vivian’s will was straightforward. The house, all its contents, and her savings were to be left to me.", NewWillNonLemma(dict.Curated()), 0)
}

func TestWillNonLemma_IgnoreWillSpecifiesEqualResponsibility(t *testing.T) {
	linttest.AssertLintCount(t, "I also reminded her that the will specifies equal responsibility", NewWillNonLemma(dict.Curated()), 0)
}

func TestWillNonLemma_IgnoreWillReflectedThat(t *testing.T) {
	linttest.AssertLintCount(t, "She meticulously planned everything, and her will reflected that.", NewWillNonLemma(dict.Curated()), 0)
}

func TestWillNonLemma_IgnoreWillStipulatesEverything(t *testing.T) {
	linttest.AssertLintCount(t, "Now, his will stipulates everything is split 50/50.", NewWillNonLemma(dict.Curated()), 0)
}
