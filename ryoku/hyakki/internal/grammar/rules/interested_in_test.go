package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestInterestedIn_FixAbout(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It suggests some useful programs the user could be interested about - NyarchLinux/NyarchWizard.", NewInterestedIn(), "It suggests some useful programs the user could be interested in - NyarchLinux/NyarchWizard.")
}

func TestInterestedIn_DontFlagAround(t *testing.T) {
	linttest.AssertLintCount(t, "We want to figure out why this is, and how to keep those interested around for longer.", NewInterestedIn(), 0)
}

func TestInterestedIn_FixAt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If someone is interested at the processed data, please email me.", NewInterestedIn(), "If someone is interested in the processed data, please email me.")
}

func TestInterestedIn_FixFor(t *testing.T) {
	t.Skip("Requires more context because 'interested for now/for sure' are not errors")
	linttest.AssertSuggestionResult(t, "but the user is only interested for the examples in one of the modes", NewInterestedIn(), "but the user is only interested in the examples in one of the modes")
}

func TestInterestedIn_DontFlagForSure(t *testing.T) {
	linttest.AssertLintCount(t, "I am interested for sure!", NewInterestedIn(), 0)
}

func TestInterestedIn_FixInto(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is one of the first complex software I wrote, and it prefigures so much of the reasons why I was interested into working on designing HTML and CSS.", NewInterestedIn(), "This is one of the first complex software I wrote, and it prefigures so much of the reasons why I was interested in working on designing HTML and CSS.")
}

func TestInterestedIn_FixOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If you are interested of tinkering.", NewInterestedIn(), "If you are interested in tinkering.")
}

func TestInterestedIn_FixOn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The creator of Photopea, a great free alternative to Photoshop, is not interested on making an offline version, so I took it upon myself to make it.", NewInterestedIn(), "The creator of Photopea, a great free alternative to Photoshop, is not interested in making an offline version, so I took it upon myself to make it.")
}

func TestInterestedIn_DontFlagThrough(t *testing.T) {
	linttest.AssertLintCount(t, "I'm happy to help walk anyone interested through doing this", NewInterestedIn(), 0)
}

func TestInterestedIn_DoesNotFlagTo(t *testing.T) {
	linttest.AssertLintCount(t, "Hi, As title suggest i am interested to know if we can run a custom model trained on yolov9 inference running on two GPU-s.", NewInterestedIn(), 0)
}

func TestInterestedIn_FixWith(t *testing.T) {
	linttest.AssertSuggestionResult(t, "no_std support (is anybody interested with this?)", NewInterestedIn(), "no_std support (is anybody interested in this?)")
}

func TestInterestedIn_DontFlagWithin(t *testing.T) {
	linttest.AssertLintCount(t, "But with no one being interested within 8 months, what help would it be.", NewInterestedIn(), 0)
}
