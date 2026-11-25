package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestVeryUnique_FixVeryUnique(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I'm not sure whether Llama Stack or ollama are generating the chat completion ids, but they are not very unique.", NewVeryUnique(), []string{
		"I'm not sure whether Llama Stack or ollama are generating the chat completion ids, but they are not unique.",
	}, []string{})
}

func TestVeryUnique_FixPrettyUnique(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Numerous accounts with my exact full name/surname (which is pretty unique) has been created (most recently).", NewVeryUnique(), "Numerous accounts with my exact full name/surname (which is pretty rare) has been created (most recently).")
}

func TestVeryUnique_FixFairlyUnique(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "In browsers, the first chars are obtained from the user agent string (which is fairly unique), and the supported mimeTypes", NewVeryUnique(), []string{
		"In browsers, the first chars are obtained from the user agent string (which is unique), and the supported mimeTypes",
	}, []string{})
}

func TestVeryUnique_FixSomewhatUnique(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A new pack of somewhat unique upgrades for R.E.P.O.!", NewVeryUnique(), "A new pack of somewhat unusual upgrades for R.E.P.O.!")
}

func TestVeryUnique_FixQuiteUnique(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Now I understand that this is quite unique to insta and if it's not useful I am also going to investigate alternatives", NewVeryUnique(), []string{
		"Now I understand that this is unique to insta and if it's not useful I am also going to investigate alternatives",
	}, []string{})
}

func TestVeryUnique_FixRatherUnique(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I regret using the Vue compiler because the resulting AST is rather unique.", NewVeryUnique(), "I regret using the Vue compiler because the resulting AST is rather unusual.")
}
