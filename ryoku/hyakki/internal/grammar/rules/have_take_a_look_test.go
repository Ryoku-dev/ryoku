package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestHaveTakeALook_CorrectTakingALook(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Consider taking a look at crossorigin attribute.", NewHaveTakeALook(dict.British), "Consider having a look at crossorigin attribute.")
}

func TestHaveTakeALook_CorrectTakeALook(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Have time to help take a look at the jdk21 upgrade issue.", NewHaveTakeALook(dict.Australian), "Have time to help have a look at the jdk21 upgrade issue.")
}

func TestHaveTakeALook_CorrectHaveALook(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Have a look at this question crashing histoire using init-state and ref.", NewHaveTakeALook(dict.American), "Take a look at this question crashing histoire using init-state and ref.")
}

func TestHaveTakeALook_CorrectTakenALook(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Have you taken a look at HQEMU?", NewHaveTakeALook(dict.British), "Have you had a look at HQEMU?")
}

func TestHaveTakeALook_CorrectHadALook(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I had a look at the podman.go and have some theories I could test.", NewHaveTakeALook(dict.Canadian), "I took a look at the podman.go and have some theories I could test.")
}

func TestHaveTakeALook_CorrectTookALook(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I though GitHub's “Dashboard” page might help with this, so I took a look.", NewHaveTakeALook(dict.Australian), "I though GitHub's “Dashboard” page might help with this, so I had a look.")
}

func TestHaveTakeALook_CorrectTakesALook(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm closing this one, but it would be nice if someone takes a look at the notes in the original issue.", NewHaveTakeALook(dict.British), "I'm closing this one, but it would be nice if someone has a look at the notes in the original issue.")
}

func TestHaveTakeALook_CorrectHavingALook(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It only appeared after I was having a look through the files.", NewHaveTakeALook(dict.American), "It only appeared after I was taking a look through the files.")
}

func TestHaveTakeALook_CorrectHasALook(t *testing.T) {
	linttest.AssertSuggestionResult(t, "When Serializing messages the code in SchemaRegistrySerde has a look into the registry using the topic name.", NewHaveTakeALook(dict.Canadian), "When Serializing messages the code in SchemaRegistrySerde takes a look into the registry using the topic name.")
}
