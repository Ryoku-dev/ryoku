package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestBarelyUn_FixUnafraid(t *testing.T) {
	linttest.AssertSuggestionResult(t, "drone footage capturing me barely unafraid of rattlesnakes", NewBarelyUn(dict.Curated()), "drone footage capturing me barely afraid of rattlesnakes")
}

func TestBarelyUn_FixUnaware(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Many Christians claim to be followers of Jesus Christ but wander in the darkness barely unaware of the dangerous ramifications", NewBarelyUn(dict.Curated()), "Many Christians claim to be followers of Jesus Christ but wander in the darkness barely aware of the dangerous ramifications")
}

func TestBarelyUn_FixUnbelievable(t *testing.T) {
	linttest.AssertSuggestionResult(t, "with the locals then taking a barely unbelievable lead when Romarinho netted five minutes before half-time", NewBarelyUn(dict.Curated()), "with the locals then taking a barely believable lead when Romarinho netted five minutes before half-time")
}

func TestBarelyUn_FixUncommon(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm not seeing how this is an artifact. It's barely uncommon.", NewBarelyUn(dict.Curated()), "I'm not seeing how this is an artifact. It's barely common.")
}

func TestBarelyUn_FixUnconscious(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That night, the only dreams that troubled his barely unconscious sleeping mind, were of The Ravens Gate Bridge, all dark and foreboding.", NewBarelyUn(dict.Curated()), "That night, the only dreams that troubled his barely conscious sleeping mind, were of The Ravens Gate Bridge, all dark and foreboding.")
}

func TestBarelyUn_FixUnconsciousCapitalized(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I remember sessions of my youth when I was dropped barely unconscious and sat out the rest of the battle", NewBarelyUn(dict.Curated()), "I remember sessions of my youth when I was dropped barely conscious and sat out the rest of the battle")
}

func TestBarelyUn_FixUnhealthy(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This could be explained by the fact that the administration of a large number of cells in a barely unhealthy animal might trigger an immune response", NewBarelyUn(dict.Curated()), "This could be explained by the fact that the administration of a large number of cells in a barely healthy animal might trigger an immune response")
}

func TestBarelyUn_FixUnknown(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the electrocatalytic activity in CNFs is barely unknown", NewBarelyUn(dict.Curated()), "the electrocatalytic activity in CNFs is barely known")
}

func TestBarelyUn_FixUnlikable(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's confusing, weird with barely unlikable characters.", NewBarelyUn(dict.Curated()), "It's confusing, weird with barely likable characters.")
}

func TestBarelyUn_FixUnlivable(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So yes the hotel isnt 5* but its barely unlivable!", NewBarelyUn(dict.Curated()), "So yes the hotel isnt 5* but its barely livable!")
}
