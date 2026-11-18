package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestTryOnesLuck_FixTriedHis(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Ndabayithethwa Ndlondlo tried out his luck from distance.", NewTryOnesLuck(), "Ndabayithethwa Ndlondlo tried his luck from distance.")
}

func TestTryOnesLuck_FixTriedMy(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Tried out my luck on making other people OCs if I made them", NewTryOnesLuck(), "Tried my luck on making other people OCs if I made them")
}

func TestTryOnesLuck_FixTriedTheir(t *testing.T) {
	linttest.AssertSuggestionResult(t, "However a few homelab'ers on the ServeTheHome Forum just tried out their luck to figure out how to utilize these boards", NewTryOnesLuck(), "However a few homelab'ers on the ServeTheHome Forum just tried their luck to figure out how to utilize these boards")
}

func TestTryOnesLuck_FixTriesOutHis(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Inggo Tries Out His Luck in Politics", NewTryOnesLuck(), "Inggo Tries His Luck in Politics")
}

func TestTryOnesLuck_FixTryOutHer(t *testing.T) {
	linttest.AssertSuggestionResult(t, "this woman had deigned to try out her luck at my place", NewTryOnesLuck(), "this woman had deigned to try her luck at my place")
}

func TestTryOnesLuck_FixTryOutHis(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Turned out to be a young American kid who decided to leave Washington DC and come try out his luck in Shanghai.", NewTryOnesLuck(), "Turned out to be a young American kid who decided to leave Washington DC and come try his luck in Shanghai.")
}

func TestTryOnesLuck_FixTryOutMy(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So I am just gonna try out my luck here with a problem that I have been facing with my prod ETL project.", NewTryOnesLuck(), "So I am just gonna try my luck here with a problem that I have been facing with my prod ETL project.")
}

func TestTryOnesLuck_FixTryOutOur(t *testing.T) {
	linttest.AssertSuggestionResult(t, "we decided to try out our luck and challenge the mighty Himalayas", NewTryOnesLuck(), "we decided to try our luck and challenge the mighty Himalayas")
}

func TestTryOnesLuck_FixTryOutTheir(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Users will come over to try out their luck, see what is success rate", NewTryOnesLuck(), "Users will come over to try their luck, see what is success rate")
}

func TestTryOnesLuck_FixTryOutYour(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Try out your luck.", NewTryOnesLuck(), "Try your luck.")
}

func TestTryOnesLuck_FixTryingHer(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A story about a girl in Chemistry trying out her luck and commitment towards this discipline in an attempt to get a Ph.D.", NewTryOnesLuck(), "A story about a girl in Chemistry trying her luck and commitment towards this discipline in an attempt to get a Ph.D.")
}

func TestTryOnesLuck_FixTryingHis(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A simple man trying out his luck in this probabilistic world.", NewTryOnesLuck(), "A simple man trying his luck in this probabilistic world.")
}

func TestTryOnesLuck_FixTryingMy(t *testing.T) {
	linttest.AssertSuggestionResult(t, "No need to be sorry hahaha, I was just trying out my luck.", NewTryOnesLuck(), "No need to be sorry hahaha, I was just trying my luck.")
}

func TestTryOnesLuck_FixTryingOur(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's now late 2016, almost two years of trying out our luck in the business world with Mike, I called it quits.", NewTryOnesLuck(), "It's now late 2016, almost two years of trying our luck in the business world with Mike, I called it quits.")
}

func TestTryOnesLuck_FixTryingTheir(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It was a time before millions began trying out their luck, a time when just applying guaranteed you would get a winning entry.", NewTryOnesLuck(), "It was a time before millions began trying their luck, a time when just applying guaranteed you would get a winning entry.")
}

func TestTryOnesLuck_FixTryingOutYour(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You should start by trying out your luck and come back with specific questions.", NewTryOnesLuck(), "You should start by trying your luck and come back with specific questions.")
}
