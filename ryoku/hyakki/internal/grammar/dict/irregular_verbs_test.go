package dict

import "testing"

func TestCanFindIrregularPastParticipleForPreteriteLowercase(t *testing.T) {
	got, ok := CuratedIrregularVerbs().GetPastParticipleForPreterite("arose")
	if !ok || got != "arisen" {
		t.Fatalf("got (%q, %v), want Some(arisen)", got, ok)
	}
}

func TestCanFindIrregularPastParticipleForPreteriteUppercase(t *testing.T) {
	got, ok := CuratedIrregularVerbs().GetPastParticipleForPreterite("WENT")
	if !ok || got != "gone" {
		t.Fatalf("got (%q, %v), want Some(gone)", got, ok)
	}
}

func TestCanFindIrregularPastParticipleSameAsPastTense(t *testing.T) {
	got, ok := CuratedIrregularVerbs().GetPastParticipleForPreterite("taught")
	if !ok || got != "taught" {
		t.Fatalf("got (%q, %v), want Some(taught)", got, ok)
	}
}

func TestCantFindRegularPastParticiple(t *testing.T) {
	if got, ok := CuratedIrregularVerbs().GetPastParticipleForPreterite("walked"); ok {
		t.Fatalf("got Some(%q), want None", got)
	}
}

func TestCantFindNonVerb(t *testing.T) {
	if got, ok := CuratedIrregularVerbs().GetPastParticipleForPreterite("the"); ok {
		t.Fatalf("got Some(%q), want None", got)
	}
}
