package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestGonnaGottaWanna_FixGonna(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We think you're gonna like it here.", NewGonnaGottaWannaLintGroup(dict.American), "We think you're going to like it here.")
}

func TestGonnaGottaWanna_FixGonnaTo(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I am gonna to create the region by clicking and dragging the mouse on waveform but I couldn't find the way to do that.", NewGonnaGottaWannaLintGroup(dict.American), []string{

		"I am gonna create the region by clicking and dragging the mouse on waveform but I couldn't find the way to do that.",

		"I am going to create the region by clicking and dragging the mouse on waveform but I couldn't find the way to do that.",
	}, []string{})
}

func TestGonnaGottaWanna_FixGotta(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Gotta Hear Them All: Towards Sound Source Aware Audio Generation.", NewGonnaGottaWannaLintGroup(dict.American), "Got to Hear Them All: Towards Sound Source Aware Audio Generation.")
}

func TestGonnaGottaWanna_FixGottaAError(t *testing.T) {
	linttest.AssertSuggestionResult(t, "when I try to create a c/c++ database,I gotta a error", NewGonnaGottaWannaLintGroup(dict.American), "when I try to create a c/c++ database,I got an error")
}

func TestGonnaGottaWanna_FixGottaACoupom(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You gotta a 20% OFF coupom.", NewGonnaGottaWannaLintGroup(dict.American), "You got a 20% OFF coupom.")
}

func TestGonnaGottaWanna_FixGottaTo(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "it works well enough for me and gotta to get back working on other stuff", NewGonnaGottaWannaLintGroup(dict.American), []string{

		"it works well enough for me and gotta get back working on other stuff",

		"it works well enough for me and have to get back working on other stuff",
	}, []string{})
}

func TestGonnaGottaWanna_FixWannaControl(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I wanna control G1 arms while walking using gamepad. But ...", NewGonnaGottaWannaLintGroup(dict.American), "I want to control G1 arms while walking using gamepad. But ...")
}

func TestGonnaGottaWanna_FixWannaA(t *testing.T) {
	linttest.AssertSuggestionResult(t, "And then I remember from the beginning I just wanna a very simple array with some simple opreations.", NewGonnaGottaWannaLintGroup(dict.American), "And then I remember from the beginning I just want a very simple array with some simple opreations.")
}

func TestGonnaGottaWanna_FixWannaTo(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "it is stoped but i wanna to continue", NewGonnaGottaWannaLintGroup(dict.American), []string{

		"it is stoped but i wanna continue",

		"it is stoped but i want to continue",
	}, []string{})
}

func TestGonnaGottaWanna_GonnaNoneMsg(t *testing.T) {
	linttest.AssertLintMessage(t, "gonna", NewGonnaGottaWannaLintGroup(dict.American), "`gonna` is very informal.")
}

func TestGonnaGottaWanna_GonnaAMsg(t *testing.T) {
	linttest.AssertLintMessage(t, "gonna a", NewGonnaGottaWannaLintGroup(dict.American), "`gonna` is very informal and the final `a` means `to`.")
}

func TestGonnaGottaWanna_GonnaToMsg(t *testing.T) {
	linttest.AssertLintMessage(t, "gonna to", NewGonnaGottaWannaLintGroup(dict.American), "`gonna` is very informal and the final `a` means `to`.")
}

func TestGonnaGottaWanna_GottaNounMsg(t *testing.T) {
	linttest.AssertLintMessage(t, "gotta", NewGonnaGottaWannaLintGroup(dict.American), "`gotta` is very informal.")
}

func TestGonnaGottaWanna_GottaAMsg(t *testing.T) {
	linttest.AssertLintMessage(t, "gotta a", NewGonnaGottaWannaLintGroup(dict.American), "`gotta` is very informal and the final `a` means `to`.")
}

func TestGonnaGottaWanna_GottaToMsg(t *testing.T) {
	linttest.AssertLintMessage(t, "gotta to", NewGonnaGottaWannaLintGroup(dict.American), "`gotta` is very informal and the final `a` means `to`.")
}

func TestGonnaGottaWanna_WannaNoneMsg(t *testing.T) {
	linttest.AssertLintMessage(t, "wanna", NewGonnaGottaWannaLintGroup(dict.American), "`wanna` is very informal.")
}

func TestGonnaGottaWanna_WannaAMsg(t *testing.T) {
	linttest.AssertLintMessage(t, "wanna a", NewGonnaGottaWannaLintGroup(dict.American), "`wanna` is very informal and the final `a` means `to`.")
}

func TestGonnaGottaWanna_WannaToMsg(t *testing.T) {
	linttest.AssertLintMessage(t, "wanna to", NewGonnaGottaWannaLintGroup(dict.American), "`wanna` is very informal and the final `a` means `to`.")
}
