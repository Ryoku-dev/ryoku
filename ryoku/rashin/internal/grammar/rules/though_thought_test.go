package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestThoughThought_FixIThoughI(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Looking at those I though I had to draw imgui into separate renderpass", NewThoughThought(), "Looking at those I thought I had to draw imgui into separate renderpass")
}

func TestThoughThought_FixIThoughIt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "and I though it was a shame because the data it provides can be ...", NewThoughThought(), "and I thought it was a shame because the data it provides can be ...")
}

func TestThoughThought_FixIThoughThat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I though that there may be other solutions as in other here", NewThoughThought(), "I thought that there may be other solutions as in other here")
}

func TestThoughThought_FixIThoughThey(t *testing.T) {
	linttest.AssertSuggestionResult(t, "path parsin error ( i though they were extincted )", NewThoughThought(), "path parsin error ( i thought they were extincted )")
}

func TestThoughThought_FixIThoughWe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "and that way I though we need something universial", NewThoughThought(), "and that way I thought we need something universial")
}

func TestThoughThought_FixIThoughYou(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I though you resolved the issue, so I updated my version", NewThoughThought(), "I thought you resolved the issue, so I updated my version")
}

func TestThoughThought_DontFlagItThoughI(t *testing.T) {
	linttest.AssertNoLints(t, "am including it though i believe it's nto the case because before this", NewThoughThought())
}

func TestThoughThought_DontFlagItThoughIt(t *testing.T) {
	linttest.AssertNoLints(t, "Prisma works with it though it is not officially supported by Prisma yet.", NewThoughThought())
}

func TestThoughThought_FixYouThoughIt(t *testing.T) {
	t.Skip("TODO: Can't check because `it` is both subject and object")
	linttest.AssertSuggestionResult(t, "it may reveal that the bug is not where you though it was", NewThoughThought(), "it may reveal that the bug is not where you thought it was")
}

func TestThoughThought_DontFlagYouThoughThat1(t *testing.T) {
	linttest.AssertNoLints(t, "I can tell you though that a project not using headers at all will likely be compiling much faster.", NewThoughThought())
}

func TestThoughThought_DontFlagYouThoughThat2(t *testing.T) {
	linttest.AssertNoLints(t, "I agree with you though that 2D lat/lon grids are unnecessarily confusing", NewThoughThought())
}
