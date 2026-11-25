package dict

import (
	"slices"
	"testing"
)

func assertNounFormsContain(t *testing.T, forms [][]rune, expected []rune) {
	t.Helper()
	if !slices.ContainsFunc(forms, func(form []rune) bool { return slices.Equal(form, expected) }) {
		t.Fatalf("forms %q do not contain %q", forms, expected)
	}
}

func TestRegularPluralsAddS(t *testing.T) {
	plurals := GetPlurals(Curated(), []rune{'c', 'a', 't'})
	assertNounFormsContain(t, plurals, []rune{'c', 'a', 't', 's'})
}

func TestRegularPluralsYToIes(t *testing.T) {
	plurals := GetPlurals(Curated(), []rune{'c', 'i', 't', 'y'})
	assertNounFormsContain(t, plurals, []rune{'c', 'i', 't', 'i', 'e', 's'})
}

func TestRegularPluralsVowelYAddsS(t *testing.T) {
	plurals := GetPlurals(Curated(), []rune{'b', 'o', 'y'})
	assertNounFormsContain(t, plurals, []rune{'b', 'o', 'y', 's'})
}

func TestRegularPluralsFeToVes(t *testing.T) {
	plurals := GetPlurals(Curated(), []rune{'w', 'i', 'f', 'e'})
	assertNounFormsContain(t, plurals, []rune{'w', 'i', 'v', 'e', 's'})
}

func TestRegularPluralsFToVes(t *testing.T) {
	plurals := GetPlurals(Curated(), []rune{'w', 'o', 'l', 'f'})
	assertNounFormsContain(t, plurals, []rune{'w', 'o', 'l', 'v', 'e', 's'})
}

func TestRegularPluralsEsEndings(t *testing.T) {
	plurals := GetPlurals(Curated(), []rune{'b', 'u', 's'})
	assertNounFormsContain(t, plurals, []rune{'b', 'u', 's', 'e', 's'})
}

func TestRegularPluralsOToOes(t *testing.T) {
	plurals := GetPlurals(Curated(), []rune{'p', 'o', 't', 'a', 't', 'o'})
	assertNounFormsContain(t, plurals, []rune{'p', 'o', 't', 'a', 't', 'o', 'e', 's'})
}

func TestRegularPluralsVowelOAddsS(t *testing.T) {
	plurals := GetPlurals(Curated(), []rune{'r', 'a', 'd', 'i', 'o'})
	assertNounFormsContain(t, plurals, []rune{'r', 'a', 'd', 'i', 'o', 's'})
}

func TestRegularNounsSingleLetterWords(t *testing.T) {
	plurals := GetPlurals(Curated(), []rune{'a'})
	if len(plurals) != 0 {
		t.Fatalf("expected empty plurals, got %q", plurals)
	}
	singulars := GetSingulars(Curated(), []rune{'s'})
	if len(singulars) != 0 {
		t.Fatalf("expected empty singulars, got %q", singulars)
	}
}

func TestRegularNounsEmptyString(t *testing.T) {
	plurals := GetPlurals(Curated(), []rune{})
	if len(plurals) != 0 {
		t.Fatalf("expected empty plurals, got %q", plurals)
	}
	singulars := GetSingulars(Curated(), []rune{})
	if len(singulars) != 0 {
		t.Fatalf("expected empty singulars, got %q", singulars)
	}
}

func TestRegularNounsCharsVersions(t *testing.T) {
	plurals := GetPlurals(Curated(), []rune{'c', 'i', 't', 'y'})
	assertNounFormsContain(t, plurals, []rune{'c', 'i', 't', 'i', 'e', 's'})
	singulars := GetSingulars(Curated(), []rune{'c', 'i', 't', 'i', 'e', 's'})
	assertNounFormsContain(t, singulars, []rune{'c', 'i', 't', 'y'})
}

func TestSingularFromPluralRemoveS(t *testing.T) {
	singulars := GetSingulars(Curated(), []rune{'c', 'a', 't', 's'})
	assertNounFormsContain(t, singulars, []rune{'c', 'a', 't'})
}

func TestSingularFromPluralIesToY(t *testing.T) {
	singulars := GetSingulars(Curated(), []rune{'c', 'i', 't', 'i', 'e', 's'})
	assertNounFormsContain(t, singulars, []rune{'c', 'i', 't', 'y'})
}

func TestSingularFromPluralVesToFe(t *testing.T) {
	singulars := GetSingulars(Curated(), []rune{'w', 'i', 'v', 'e', 's'})
	assertNounFormsContain(t, singulars, []rune{'w', 'i', 'f', 'e'})
}

func TestSingularFromPluralVesToF(t *testing.T) {
	singulars := GetSingulars(Curated(), []rune{'w', 'o', 'l', 'v', 'e', 's'})
	assertNounFormsContain(t, singulars, []rune{'w', 'o', 'l', 'f'})
}

func TestSingularFromPluralEsRemoval(t *testing.T) {
	singulars := GetSingulars(Curated(), []rune{'c', 'l', 'a', 's', 's', 'e', 's'})
	assertNounFormsContain(t, singulars, []rune{'c', 'l', 'a', 's', 's'})
}

func TestSingularFromPluralOesToO(t *testing.T) {
	singulars := GetSingulars(Curated(), []rune{'p', 'o', 't', 'a', 't', 'o', 'e', 's'})
	assertNounFormsContain(t, singulars, []rune{'p', 'o', 't', 'a', 't', 'o'})
}

func TestRegularNounsEmptyStringPlural(t *testing.T) {
	plurals := GetPlurals(Curated(), []rune{})
	if len(plurals) != 0 {
		t.Fatalf("expected empty plurals, got %q", plurals)
	}
}

func TestRegularNounsEmptyStringSingular(t *testing.T) {
	singulars := GetSingulars(Curated(), []rune{})
	if len(singulars) != 0 {
		t.Fatalf("expected empty singulars, got %q", singulars)
	}
}

func TestCaseInsensitivePlurals(t *testing.T) {
	lowercaseCity := []rune("city")
	plurals := GetPlurals(Curated(), lowercaseCity)
	assertNounFormsContain(t, plurals, []rune{'c', 'i', 't', 'i', 'e', 's'})
	mixedCity := []rune("City")
	plurals = GetPlurals(Curated(), mixedCity)
	assertNounFormsContain(t, plurals, []rune{'C', 'i', 't', 'i', 'e', 's'})
}

func TestCaseInsensitiveSingulars(t *testing.T) {
	lowercaseCities := []rune("cities")
	singulars := GetSingulars(Curated(), lowercaseCities)
	assertNounFormsContain(t, singulars, []rune{'c', 'i', 't', 'y'})
	mixedCities := []rune("Cities")
	singulars = GetSingulars(Curated(), mixedCities)
	assertNounFormsContain(t, singulars, []rune{'C', 'i', 't', 'y'})
}

func TestGeneratedCandidatesMustValidate(t *testing.T) {
	plurals := GetPlurals(Curated(), []rune{'a', 'n', 't'})
	assertNounFormsContain(t, plurals, []rune{'a', 'n', 't', 's'})
	plurals = GetPlurals(Curated(), []rune{'b', 'u', 's'})
	assertNounFormsContain(t, plurals, []rune{'b', 'u', 's', 'e', 's'})
}
