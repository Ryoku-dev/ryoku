package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestGoToWar_GoAt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "specialization makes you vulnerable if you go at war with your trading partners", NewGoToWar(), "specialization makes you vulnerable if you go to war with your trading partners")
}

func TestGoToWar_GoIn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "for whatever reason, it would go in war with another town", NewGoToWar(), "for whatever reason, it would go to war with another town")
}

func TestGoToWar_GoOn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How much time do we have before Youtube starts to go on war with Revanced?", NewGoToWar(), "How much time do we have before Youtube starts to go to war with Revanced?")
}

func TestGoToWar_GoesOn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It would be the same case if USA goes on war with Canada and Mexico.", NewGoToWar(), "It would be the same case if USA goes to war with Canada and Mexico.")
}

func TestGoToWar_GoingAt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So instead of going at war with technology, let's be friends and work better.", NewGoToWar(), "So instead of going to war with technology, let's be friends and work better.")
}

func TestGoToWar_GoingOn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How consequences of India going on war with Pakistan after the recent Uri Terror attack?", NewGoToWar(), "How consequences of India going to war with Pakistan after the recent Uri Terror attack?")
}

func TestGoToWar_WentAt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the magic energy released since the colleges went at war with each others", NewGoToWar(), "the magic energy released since the colleges went to war with each others")
}

func TestGoToWar_WentIn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "even America wanted to expand its territories and they went in War with Mexico", NewGoToWar(), "even America wanted to expand its territories and they went to War with Mexico")
}

func TestGoToWar_WentOn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I used to skip clubman and make as many vills as i can and then went on war with ai", NewGoToWar(), "I used to skip clubman and make as many vills as i can and then went to war with ai")
}
