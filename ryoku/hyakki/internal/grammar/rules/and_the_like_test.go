package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAndTheLike_DontFlagAndTheLike(t *testing.T) {
	linttest.AssertNoLints(t, "The color of brackets and the like appears to be incorrect ...", NewAndTheLike())
}

func TestAndTheLike_DontFlagOrTheLike(t *testing.T) {
	linttest.AssertNoLints(t, "Does WCAG apply only to English (or the like), or does it aim to cover all languages?", NewAndTheLike())
}

func TestAndTheLike_FlagAnTheLikes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Allow jsSourceDir (an the likes) to refer to the project root. #5", NewAndTheLike(), "Allow jsSourceDir (and the like) to refer to the project root. #5")
}

func TestAndTheLike_FlagAndAlike(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Latest release breaks FilePicker and alike", NewAndTheLike(), "Latest release breaks FilePicker and the like")
}

func TestAndTheLike_FlagAndAlikes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Compiled functions (and alikes) need to keep references for their module objects", NewAndTheLike(), "Compiled functions (and the like) need to keep references for their module objects")
}

func TestAndTheLike_FlagAndTheAlike(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Suggestions, comments and the alike are welcome on http://waa.ai/4xtC", NewAndTheLike(), "Suggestions, comments and the like are welcome on http://waa.ai/4xtC")
}

func TestAndTheLike_FlagAndTheLikes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Don't report \"expected semicolon or line break\", \"expected comma\" and the likes at every token boundary", NewAndTheLike(), "Don't report \"expected semicolon or line break\", \"expected comma\" and the like at every token boundary")
}

func TestAndTheLike_FlagOrAlike(t *testing.T) {
	linttest.AssertSuggestionResult(t, "enable biome extension to \"monitor or alike\" the workspace.", NewAndTheLike(), "enable biome extension to \"monitor or the like\" the workspace.")
}

func TestAndTheLike_FlagOrAlikes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Persistent Compiler Caching with ccache or alikes", NewAndTheLike(), "Persistent Compiler Caching with ccache or the like")
}

func TestAndTheLike_FlagOrTheLikes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Description of the problem: Implement aria2c or the likes to resume partial downloads.", NewAndTheLike(), "Description of the problem: Implement aria2c or the like to resume partial downloads.")
}
