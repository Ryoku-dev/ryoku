package dict

import (
	"reflect"
	"slices"
	"testing"
)

func assertCorrection(t *testing.T, w, correct string) {
	t.Helper()
	r := SuggestCorrectSpellingStr(w, 200, 2, Curated())
	if !slices.Contains(r[:min(3, len(r))], correct) {
		t.Fatalf("%q not in top 3: %v", correct, r)
	}
}
func TestSpellNormalizesWeve(t *testing.T) {
	if string(normalized([]rune("we’ve"))) != "we've" {
		t.Fatal("normalization mismatch")
	}
}
func TestSpellPunctationNoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, w := range SuggestCorrectSpellingStr("punctation", 200, 2, Curated()) {
		if seen[w] {
			t.Fatal("duplicate suggestion")
		}
		seen[w] = true
	}
}
func TestSpellYoureContraction(t *testing.T) { assertCorrection(t, "youre", "you're") }
func TestSpellThatsContraction(t *testing.T) { assertCorrection(t, "thats", "that's") }
func TestSpellWeveContraction(t *testing.T)  { assertCorrection(t, "weve", "we've") }
func TestSpellThisCorrection(t *testing.T)   { assertCorrection(t, "ths", "this") }
func TestSpellIssue624NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, w := range SuggestCorrectSpellingStr("Semantical", 200, 2, Curated()) {
		if seen[w] {
			t.Fatal("duplicate suggestion")
		}
		seen[w] = true
	}
}
func TestSpellIssue182(t *testing.T)           { assertCorrection(t, "Im", "I'm") }
func TestSpellFstSpellcheckHvllo(t *testing.T) { assertCorrection(t, "hvllo", "hello") }
func TestSpellSpellcheckHvllo(t *testing.T)    { assertCorrection(t, "hvllo", "hello") }
func TestSpellSpellcheckAout(t *testing.T)     { assertCorrection(t, "aout", "about") }
func TestSpellSpellcheckingIsDeterministic(t *testing.T) {
	a := SuggestCorrectSpellingStr("hello", 200, 2, Curated())
	for range 2 {
		if !reflect.DeepEqual(a, SuggestCorrectSpellingStr("hello", 200, 2, Curated())) {
			t.Fatal("nondeterministic")
		}
	}
}
func TestSpellAdvicedCorrection(t *testing.T)     { assertCorrection(t, "adviced", "advised") }
func TestSpellAknowledgedCorrection(t *testing.T) { assertCorrection(t, "aknowledged", "acknowledged") }
func TestSpellAlcaholicCorrection(t *testing.T)   { assertCorrection(t, "alcaholic", "alcoholic") }
func TestSpellSlavesCorrection(t *testing.T)      { assertCorrection(t, "Slaves", "Slavs") }
func TestSpellConciousnessCorrection(t *testing.T) {
	assertCorrection(t, "conciousness", "consciousness")
}
func TestSpellVApostropheSSuggestsVs(t *testing.T)              { assertCorrection(t, "v's", "vs") }
func TestSpellVApostropheTypographicalSSuggestsVs(t *testing.T) { assertCorrection(t, "v’s", "vs") }
func TestSpellMissingApostropheChildrensSuggestsChildrens(t *testing.T) {
	assertCorrection(t, "childrens", "children's")
}
