package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestPossessiveYour_YourComments(t *testing.T) {
	t.Skip("Upstream ignored: currently fails because comments is a homographs (verb or noun)")
	t.Skip("currently fails because comments is a homographs (verb or noun)")
	linttest.AssertSuggestionResult(t, "You comments may end up in the documentation.", NewPossessiveYour(), "Your comments may end up in the documentation.")
}

func TestPossessiveYour_AllowIntroPage(t *testing.T) {
	linttest.AssertLintCount(t, "You can try out an editor that uses Harper under-the-hood here.", NewPossessiveYour(), 0)
}

func TestPossessiveYour_AllowYouGuys(t *testing.T) {
	linttest.AssertLintCount(t, "I mean I'm pretty sure you guys can't do anything with this stuff.", NewPossessiveYour(), 0)
}

func TestPossessiveYour_TestSuggestionYour(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You combination of artist and teacher.", NewPossessiveYour(), "Your combination of artist and teacher.")
}

func TestPossessiveYour_TestSuggestionYoureA(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You combination of artist and teacher.", NewPossessiveYour(), "You're a combination of artist and teacher.")
}

func TestPossessiveYour_TestSuggestionMultiple(t *testing.T) {
	t.Skip("Upstream ignored: ignored in Harper")
	t.Skip("Ignored by upstream Harper")
	linttest.AssertSuggestionResult(t, "You knowledge. You imagination. You icosahedron", NewPossessiveYour(), "Your knowledge. Your imagination. You're an icosahedron")
}

func TestPossessiveYour_DontFlagJustShowingYou(t *testing.T) {
	linttest.AssertLintCount(t, "I'm just showing you what's available and how to use it.", NewPossessiveYour(), 0)
}

func TestPossessiveYour_AllowsIssue1583(t *testing.T) {
	linttest.AssertNoLints(t, "Note that in a world with modules everywhere, you almost never need an IIFE", NewPossessiveYour())
}

func TestPossessiveYour_DontFlag1919BroughtYou(t *testing.T) {
	linttest.AssertLintCount(t, "team who also brought you [BloodHound Enterprise](http://specterops.io/bloodhound-verview/).", NewPossessiveYour(), 0)
}

func TestPossessiveYour_DontFlag1919TeachesYou(t *testing.T) {
	linttest.AssertLintCount(t, "Teaches you PyTorch and many machine learning concepts in a hands-on, code-first way.", NewPossessiveYour(), 0)
}
