package dict

import (
	"reflect"
	"testing"
)

const testRuneWordList = "4\nhello\ntry/B\nwork/AB\nblank/"
const testRuneWordListBlankLines = "4\n\nhello\n\ntry/B\nwork/AB\n\n\nblank/"
const testRuneWordListFullComments = "4\n#\nhello\n#with\ntry/B\nwork/AB\n# some\n# comments aded\nblank/"
const testRuneWordListComments = "4\nhello       # a word without attributes\ntry/B   \t  # a word with empty attributes\nwork/AB\t   #a word with one attribute\nblank/      #a word with two attributes"
const testRuneAffixJSON = `{
 "affixes":{
  "A":{"kind":"prefix","cross_product":true,"replacements":[{"remove":"","add":"re","condition":"."}],"target":[],"base_metadata":{}},
  "B":{"kind":"suffix","cross_product":true,"replacements":[{"remove":"","add":"ed","condition":"[^y]"},{"remove":"y","add":"ied","condition":"y"}],"target":[{"metadata":{"noun":{}}}],"base_metadata":{}}
 },"properties":{}
}`

func assertRuneExpansion(t *testing.T, source string) {
	t.Helper()
	entries, err := expandRuneFiles(source, testRuneAffixJSON)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, entry := range entries {
		got[string(entry.CanonicalSpelling)] = true
	}
	want := map[string]bool{}
	for _, word := range []string{"reworked", "rework", "tried", "try", "hello", "worked", "work", "blank"} {
		want[word] = true
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expanded words: got %v, want %v", got, want)
	}
}
func TestCorrectlyExpandsTestFiles(t *testing.T) { assertRuneExpansion(t, testRuneWordList) }
func TestCorrectlyExpandsTestFilesWithBlankLines(t *testing.T) {
	assertRuneExpansion(t, testRuneWordListBlankLines)
}
func TestCorrectlyExpandsTestFilesWithFullLineComments(t *testing.T) {
	assertRuneExpansion(t, testRuneWordListFullComments)
}
func TestCorrectlyExpandsTestFilesWithComments(t *testing.T) {
	assertRuneExpansion(t, testRuneWordListComments)
}
func TestCanParseTestFile(t *testing.T) {
	words, err := ParseWordList(testRuneWordList)
	if err != nil {
		t.Fatal(err)
	}
	if len(words[len(words)-1].Annotations) != 0 {
		t.Fatal("last entry has annotations")
	}
	if len(words) != 4 {
		t.Fatalf("got %d words, want 4", len(words))
	}
}
func TestPluralGiants(t *testing.T) {
	source := `{"affixes":{
 "S":{"kind":"suffix","cross_product":true,"replacements":[
 {"remove":"y","add":"ies","condition":"[^aeiou]"},
 {"remove":"","add":"s","condition":"[aeiou]y"},
 {"remove":"","add":"s","condition":"[^sxzhy]"}],
 "target":[{"metadata":{"noun":{"is_plural":true}}}],"base_metadata":{"noun":{}}},
 "M":{"kind":"suffix","cross_product":true,"replacements":[{"remove":"","add":"'s","condition":"."}],"target":[],"base_metadata":{}}
 },"properties":{}}`
	words, err := NewWordMapFromRuneFiles("1\ngiant/SM", source)
	if err != nil {
		t.Fatal(err)
	}
	giant := words.GetCanonical(NewCanonicalWordIdFromWordStr("giant"))
	if giant == nil {
		t.Fatal("giant missing")
	}
	if !giant.Metadata.IsNoun() {
		t.Fatal("giant must be a noun")
	}
	giants := words.GetCanonical(NewCanonicalWordIdFromWordStr("giants"))
	if giants == nil {
		t.Fatal("giants missing")
	}
	if !giants.Metadata.IsPluralNoun() {
		t.Fatal("giants must be a plural noun")
	}
}
func TestProperNounPropertyPropagatesToPlurals(t *testing.T) {
	if metadata := Curated().GetWordMetadataStr("Volkswagens"); metadata != nil && !metadata.IsProperNoun() {
		t.Fatal("Volkswagens must be a proper noun")
	}
}
func TestProperNounPropagatesToPossessives2327(t *testing.T) {
	if metadata := Curated().GetWordMetadataStr("Volkswagen's"); metadata != nil && !metadata.IsPossessiveNoun() {
		t.Fatal("Volkswagen's must be a possessive noun")
	}
}
func runeTestMatcher(t *testing.T, source string) Matcher {
	t.Helper()
	matcher, err := ParseMatcher(source)
	if err != nil {
		t.Fatal(err)
	}
	return matcher
}
func TestParsesSimple(t *testing.T) {
	matcher := runeTestMatcher(t, "[^aeiou]a.s")
	want := []matcherOperator{{kind: matcherNone, chars: []rune{'a', 'e', 'i', 'o', 'u'}}, {kind: matcherLiteral, chars: []rune{'a'}}, {kind: matcherAny}, {kind: matcherLiteral, chars: []rune{'s'}}}
	if !reflect.DeepEqual(matcher.operators, want) {
		t.Fatalf("got %#v, want %#v", matcher.operators, want)
	}
}
func TestParsesMultiBrackets(t *testing.T) {
	matcher := runeTestMatcher(t, "u[xy]a[^z]")
	want := []matcherOperator{{kind: matcherLiteral, chars: []rune{'u'}}, {kind: matcherOne, chars: []rune{'x', 'y'}}, {kind: matcherLiteral, chars: []rune{'a'}}, {kind: matcherNone, chars: []rune{'z'}}}
	if !reflect.DeepEqual(matcher.operators, want) {
		t.Fatalf("got %#v, want %#v", matcher.operators, want)
	}
}
func TestMatchesVowels(t *testing.T) {
	matcher := runeTestMatcher(t, "[aeiou]")
	for _, vowel := range []rune{'a', 'e', 'i', 'o', 'u'} {
		if !matcher.Matches([]rune{vowel}) {
			t.Errorf("failed matching %c", vowel)
		}
	}
}
func TestMatchesMultipleBracketVowels(t *testing.T) {
	matcher := runeTestMatcher(t, "u[xy]a[^z]")
	for _, word := range []string{"uxaq", "uyay"} {
		if !matcher.Matches([]rune(word)) {
			t.Errorf("failed matching %s", word)
		}
	}
	for _, word := range []string{"ueae", "uxaz"} {
		if matcher.Matches([]rune(word)) {
			t.Errorf("unexpected match %s", word)
		}
	}
}
func TestRoundTrip(t *testing.T) {
	source := "[^aeiou]a.s"
	if got := runeTestMatcher(t, source).String(); got != source {
		t.Fatalf("got %s, want %s", got, source)
	}
}
func TestRoundTripMultipleBrackets(t *testing.T) {
	source := "über[^e]gegen[^eü]"
	if got := runeTestMatcher(t, source).String(); got != source {
		t.Fatalf("got %s, want %s", got, source)
	}
}

func TestConditionalAffixMetadataAndDerivation(t *testing.T) {
	source := `{"affixes":{"S":{"kind":"suffix","cross_product":false,"replacements":[{"remove":"","add":"s","condition":"."}],"target":[{"metadata":{"noun":{"is_plural":true}},"if_base":{"noun":{}}}],"base_metadata":{}}},"properties":{"N":{"metadata":{"noun":{}}}}}`
	words, err := NewWordMapFromRuneFiles("2\ncat/NS\nrun/S", source)
	if err != nil {
		t.Fatal(err)
	}
	cats := words.GetCanonical(NewCanonicalWordIdFromWordStr("cats"))
	if cats == nil || !cats.Metadata.IsPluralNoun() {
		t.Fatal("conditional noun metadata missing")
	}
	runs := words.GetCanonical(NewCanonicalWordIdFromWordStr("runs"))
	if runs == nil || runs.Metadata.IsNoun() {
		t.Fatal("conditional metadata applied without a noun base")
	}
	if !cats.Metadata.DerivedFrom.Contains(NewCanonicalWordIdFromWordStr("cat")) {
		t.Fatal("derivation missing")
	}
}
func TestAffixPropertyPropagation(t *testing.T) {
	source := `{"affixes":{
 "P":{"kind":"prefix","cross_product":true,"replacements":[{"remove":"","add":"re","condition":"."}],"target":[],"base_metadata":{}},
 "S":{"kind":"suffix","cross_product":true,"replacements":[{"remove":"","add":"s","condition":"."}],"target":[],"base_metadata":{}}
 },"properties":{"N":{"metadata":{"noun":{}}},"A":{"propagate":true,"metadata":{"adjective":{}}}}}`
	words, err := NewWordMapFromRuneFiles("1\nwork/PSNA", source)
	if err != nil {
		t.Fatal(err)
	}
	for _, word := range []string{"work", "rework"} {
		entry := words.GetCanonical(NewCanonicalWordIdFromWordStr(word))
		if entry == nil || !entry.Metadata.IsNoun() || !entry.Metadata.IsAdjective() {
			t.Errorf("%s lost base properties", word)
		}
	}
	for _, word := range []string{"works", "reworks"} {
		entry := words.GetCanonical(NewCanonicalWordIdFromWordStr(word))
		if entry == nil || entry.Metadata.IsNoun() || !entry.Metadata.IsAdjective() {
			t.Errorf("%s has incorrect propagated properties", word)
		}
	}
}
func TestAffixMalformedInputs(t *testing.T) {
	for _, source := range []string{"", "not a count\nword", "-1\nword", " 1\nword", "1\r"} {
		if _, err := ParseWordList(source); err == nil {
			t.Errorf("accepted invalid count %q", source)
		}
	}
	if _, err := ParseMatcher("ü[ae"); err == nil || err.Error() != "Unmatched bracket at index: 2" {
		t.Fatalf("incorrect matcher error: %v", err)
	}
	if _, err := ParseAttributeList(`{"affixes":{},"properties":{}}`); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{`{}`, `{"affixes":{"AB":{}},"properties":{}}`, `{"affixes":{},"properties":{"N":{}}}`} {
		if _, err := ParseAttributeList(source); err == nil {
			t.Errorf("accepted invalid attributes %s", source)
		}
	}
}

func TestAffixSpellingEqualUTF8Keys(t *testing.T) {
	words := [][]rune{
		nil,
		{},
		[]rune("cafe"),
		[]rune("café"),
		{'x', '\uFFFD'},
		{'x', -1},
		{'x', 0xD800},
		{'x', 0x110000},
	}
	for _, a := range words {
		for _, b := range words {
			if got, want := affixSpellingEqual(a, b), string(a) == string(b); got != want {
				t.Fatalf("equal(%v, %v) = %v; want %v", a, b, got, want)
			}
		}
	}
}

func TestWordListStreamingLineEndings(t *testing.T) {
	want := []AnnotatedWord{
		{Letters: []rune("hello"), Annotations: []rune("N")},
		{Letters: []rune("try"), Annotations: []rune("B")},
	}
	for _, source := range []string{
		"2\nhello/N\ntry/B",
		"+2\r\n# comment\r\nhello/N \t# note\r\n\r\ntry/B\r\n",
		"2\r\nhello/N\r\ntry/B\r",
	} {
		got, err := ParseWordList(source)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("ParseWordList(%q) = %v, %v; want %v", source, got, err, want)
		}
	}
}
