package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestThatThan_FixSlowerThat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Local installed PHAR 5x times slower that the same PHAR installed globally", NewThatThan(), "Local installed PHAR 5x times slower than the same PHAR installed globally")
}

func TestThatThan_DontFlagMoreThat(t *testing.T) {
	linttest.AssertNoLints(t, "so it's probably more that Croatian had an easier test", NewThatThan())
}

func TestThatThan_DontFlagEasierThatWay(t *testing.T) {
	linttest.AssertNoLints(t, "Given svelte now has signals, it might actually be easier that way.", NewThatThan())
}

func TestThatThan_DontFlagBetterThat(t *testing.T) {
	linttest.AssertNoLints(t, "So I am wondering if its better that I run SCENIC+ once on the integrated dataset or 3 times on the individual datasets", NewThatThan())
}

func TestThatThan_FixBetterThat(t *testing.T) {
	t.Skip("not handled because 'better' results in false positives")
	linttest.AssertSuggestionResult(t, "Examples of how different cards perform far better that others.", NewThatThan(), "Examples of how different cards perform far better than others.")
}

func TestThatThan_FixSmallerThat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "When the resulting part is smaller that the build plate, it gets re-arranged.", NewThatThan(), "When the resulting part is smaller than the build plate, it gets re-arranged.")
}

func TestThatThan_CantFlagBiggerThat(t *testing.T) {
	t.Skip("not handled because 'bigger' results in false positives")
	linttest.AssertSuggestionResult(t, "Enable bigger that 1024*768 window for world builder.", NewThatThan(), "Enable bigger than 1024*768 window for world builder.")
}

func TestThatThan_FixLongerThat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Window list in CodeBrowser can be longer that screen height.", NewThatThan(), "Window list in CodeBrowser can be longer than screen height.")
}

func TestThatThan_FixLessThat(t *testing.T) {
	t.Skip("'less that' also occurs in false positives")
	linttest.AssertSuggestionResult(t, "Collector Not collecting metrics if the collection interval is less that the metric generation interval.", NewThatThan(), "Collector Not collecting metrics if the collection interval is less than the metric generation interval.")
}

func TestThatThan_FixFasterThat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "with the general case performing approximately 4x faster that a Vec based implementation", NewThatThan(), "with the general case performing approximately 4x faster than a Vec based implementation")
}

func TestThatThan_FixTallerThat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Notice that people we've already placed are not taller that the current person.", NewThatThan(), "Notice that people we've already placed are not taller than the current person.")
}

func TestThatThan_DontFixFasterThatWay(t *testing.T) {
	linttest.AssertNoLints(t, "You will get an answer quicker that way!", NewThatThan())
}

func TestThatThan_DontFixLighterThat(t *testing.T) {
	linttest.AssertNoLints(t, "This is the code for Seed-Studio-based timer and desk lighter that I built as a gift for a good friend.", NewThatThan())
}

func TestThatThan_DontFlagMoreExplicitThat(t *testing.T) {
	linttest.AssertNoLints(t, "make it more explicit that those files are auto ...", NewThatThan())
}

func TestThatThan_DontFlagMoreClearThat(t *testing.T) {
	linttest.AssertNoLints(t, "Make it more clear that users need to download the VS tooling installer for .NET Core in VS.", NewThatThan())
}

func TestThatThan_DontFlagIGatheredLaterThat(t *testing.T) {
	linttest.AssertNoLints(t, "and I gathered later that he was a photographer", NewThatThan())
}

func TestThatThan_DontFlagItsBetterThat(t *testing.T) {
	linttest.AssertNoLints(t, "It’s better that the shock should all come at once.", NewThatThan())
}

func TestThatThan_DontFlagNumberThat1663(t *testing.T) {
	linttest.AssertNoLints(t, " 455 │ `MAJOR.MINOR.PATCH` version number that increments with:", NewThatThan())
}

func TestThatThan_Issue3902(t *testing.T) {
	linttest.AssertNoLints(t, "A fish steamer that spans two burners is essential.", NewThatThan())
}
