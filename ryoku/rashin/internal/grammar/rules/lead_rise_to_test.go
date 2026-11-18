package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestLeadRiseTo_FixLedSimplePast(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In this way, it led rise to a kind of monotheism in Egypt.", NewLeadRiseTo(), "In this way, it gave rise to a kind of monotheism in Egypt.")
}

func TestLeadRiseTo_FixLedPastParticiple(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This had led rise to some issues, such as #2777 and some over Slack", NewLeadRiseTo(), "This had given rise to some issues, such as #2777 and some over Slack")
}

func TestLeadRiseTo_FixLeadSpelloForLed(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This lead rise to a fair number of complaints over image quality which were not down to RPT.", NewLeadRiseTo(), "This gave rise to a fair number of complaints over image quality which were not down to RPT.")
}

func TestLeadRiseTo_FixLeadNotSpello(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Philosophy is important because it raises the questions that lead rise to the sciences.", NewLeadRiseTo(), "Philosophy is important because it raises the questions that give rise to the sciences.")
}

func TestLeadRiseTo_FixLeads(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This leads rise to another question of mine", NewLeadRiseTo(), "This gives rise to another question of mine")
}

func TestLeadRiseTo_FixLeading(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The severe bushfires have also created their own weather, leading rise to a phenomenon known as pyrocumulonimbus (pyroCB) storms.", NewLeadRiseTo(), "The severe bushfires have also created their own weather, giving rise to a phenomenon known as pyrocumulonimbus (pyroCB) storms.")
}
