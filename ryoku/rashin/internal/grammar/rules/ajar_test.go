package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAjar_FixDoorAJarHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "not simply breaking into the house of a man they suspected of a crime but also them leaving his door a-jar after they left", NewAjar(), "not simply breaking into the house of a man they suspected of a crime but also them leaving his door ajar after they left")
}

func TestAjar_FixSlightlyAJarHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A door to democratizing investing that's slightly a-jar.", NewAjar(), "A door to democratizing investing that's slightly ajar.")
}

func TestAjar_FixDoorAJarSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "so it's easier just to read my book indoors with the door a jar so ...", NewAjar(), "so it's easier just to read my book indoors with the door ajar so ...")
}

func TestAjar_FixCarDoor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Why is my car calling my door a jar? Is it stupid?", NewAjar(), "Why is my car calling my door ajar? Is it stupid?")
}

func TestAjar_FixMouth(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I dropped my book on the ground mouth a jar when it happened.", NewAjar(), "I dropped my book on the ground mouth ajar when it happened.")
}

func TestAjar_FixRationalDoors(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It keeps my rational doors a jar.", NewAjar(), "It keeps my rational doors ajar.")
}

func TestAjar_FixGate(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Leaving the garden gate a jar ...", NewAjar(), "Leaving the garden gate ajar ...")
}
