package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestThisTypeOfThing_FixThatKindOfThings(t *testing.T) {
	linttest.AssertSuggestionResult(t, "it's specific to TypeScript and not Go nor Python can do that kind of things", NewThisTypeOfThing(), "it's specific to TypeScript and not Go nor Python can do that kind of thing")
}

func TestThisTypeOfThing_FixThatSortOfThings(t *testing.T) {
	linttest.AssertSuggestionResult(t, "there isn't a trivial stb-like ready-to-use C++ library to do that sort of things", NewThisTypeOfThing(), "there isn't a trivial stb-like ready-to-use C++ library to do that sort of thing")
}

func TestThisTypeOfThing_FixTheseKindOfThings(t *testing.T) {
	linttest.AssertSuggestionResult(t, "For these kind of things, I think it would be great to have a user-defined field which can be used to search for files.", NewThisTypeOfThing(), "For these kinds of things, I think it would be great to have a user-defined field which can be used to search for files.")
}

func TestThisTypeOfThing_FixTheseSortOfThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "People from npm actually get death threats for these sort of thing", NewThisTypeOfThing(), "People from npm actually get death threats for this sort of thing")
}

func TestThisTypeOfThing_FixTheseSortOfThings(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I suppose doing these sort of things should be legal", NewThisTypeOfThing(), "I suppose doing these sorts of things should be legal")
}

func TestThisTypeOfThing_FixTheseSortsOfThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "What I would like to understand is what the syntactic structure is for these sorts of things.", NewThisTypeOfThing(), "What I would like to understand is what the syntactic structure is for these sorts of things.")
}

func TestThisTypeOfThing_FixTheseTypeOfThings(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You can use the Symfony validator to validate these type of things easily.", NewThisTypeOfThing(), "You can use the Symfony validator to validate these types of things easily.")
}

func TestThisTypeOfThing_FixThisKindOfThings(t *testing.T) {
	linttest.AssertSuggestionResult(t, "this kind of things could exists in languages like Haskell which supports higher kinded types", NewThisTypeOfThing(), "this kind of thing could exists in languages like Haskell which supports higher kinded types")
}

func TestThisTypeOfThing_FixThisSortOfThings(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have heard this sort of things happening in the movie industry but it's appalling that it happens in the business world too", NewThisTypeOfThing(), "I have heard this sort of thing happening in the movie industry but it's appalling that it happens in the business world too")
}

func TestThisTypeOfThing_FixThisTypeOfThings(t *testing.T) {
	linttest.AssertSuggestionResult(t, "how to handle this type of things", NewThisTypeOfThing(), "how to handle this type of thing")
}

func TestThisTypeOfThing_FixThoseKindOfThings(t *testing.T) {
	linttest.AssertSuggestionResult(t, "uh, so I was playing both of those kind of things", NewThisTypeOfThing(), "uh, so I was playing both of those kinds of things")
}
