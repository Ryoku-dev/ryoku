package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestLittleKnown_FixFactNeedsArticleWithSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's little known fact that Legolas did have a dialogue in the Frodo waking up scene", NewLittleKnown(), "It's a little known fact that Legolas did have a dialogue in the Frodo waking up scene")
}

func TestLittleKnown_FixFactNeedsArticleWithHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's little-known fact among family and close friends, but it doesn't really come up.", NewLittleKnown(), "It's a little-known fact among family and close friends, but it doesn't really come up.")
}

func TestLittleKnown_FixThatShouldntHaveArticleHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In the Macedonian and in the Turkish scientific public, it's a little-known that Mustafa Kemal visited Shtip in july 1909.", NewLittleKnown(), "In the Macedonian and in the Turkish scientific public, it's little-known that Mustafa Kemal visited Shtip in july 1909.")
}

func TestLittleKnown_FixThatShouldntHaveArticleSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "And so it's a little known that the first post-war chancellor of Germany, Konrad Adenauer, who took charge of the democratic rebirth of Germany ...", NewLittleKnown(), "And so it's little known that the first post-war chancellor of Germany, Konrad Adenauer, who took charge of the democratic rebirth of Germany ...")
}
