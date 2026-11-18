package dict

import "testing"

func TestCanFindIrregularPluralForSingularLowercase(t *testing.T) {
	got, ok := CuratedIrregularNouns().GetPluralForSingular("man")
	if !ok || got != "men" {
		t.Fatalf("got (%q, %v), want Some(men)", got, ok)
	}
}

func TestCanFindIrregularPluralForSingularUppercase(t *testing.T) {
	got, ok := CuratedIrregularNouns().GetPluralForSingular("WOMAN")
	if !ok || got != "women" {
		t.Fatalf("got (%q, %v), want Some(women)", got, ok)
	}
}

func TestCanFindSingularForIrregularPlural(t *testing.T) {
	got, ok := CuratedIrregularNouns().GetSingularForPlural("children")
	if !ok || got != "child" {
		t.Fatalf("got (%q, %v), want Some(child)", got, ok)
	}
}

func TestCantFindRegularPlural(t *testing.T) {
	if got, ok := CuratedIrregularNouns().GetPluralForSingular("car"); ok {
		t.Fatalf("got Some(%q), want None", got)
	}
}

func TestCantFindNonNoun(t *testing.T) {
	if got, ok := CuratedIrregularNouns().GetPluralForSingular("the"); ok {
		t.Fatalf("got Some(%q), want None", got)
	}
}
