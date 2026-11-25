package dict

import (
	"reflect"
	"slices"
	"sort"
	"testing"
)

func assertUniqueWords(t *testing.T, d Dictionary) {
	t.Helper()
	seen := map[string]bool{}
	for _, w := range d.WordsIter() {
		s := string(w)
		if seen[s] {
			t.Fatalf("duplicate %q", s)
		}
		seen[s] = true
	}
}
func assertSortedFuzzy(t *testing.T, d Dictionary) {
	t.Helper()
	r := d.FuzzyMatchStr("hello", 3, 100)
	for i := 1; i < len(r); i++ {
		if r[i-1].EditDistance > r[i].EditDistance {
			t.Fatal("unsorted edit distances")
		}
	}
}
func checkContains(t *testing.T, d Dictionary, words ...string) {
	t.Helper()
	for _, w := range words {
		if !d.ContainsWordStr(w) {
			t.Fatalf("missing %q", w)
		}
	}
}
func TestMutableCuratedContainsNoDuplicates(t *testing.T) {
	assertUniqueWords(t, Curated().MutableDictionary)
}
func TestMutableCuratedMatchesCapitalized(t *testing.T) { checkContains(t, Curated(), "this", "This") }
func TestMutableThisIsDeterminer(t *testing.T) {
	if !testMD(t, "this").IsDeterminer() || !testMD(t, "This").IsDeterminer() {
		t.Fatal("not determiner")
	}
}
func TestMutableSeveralIsQuantifier(t *testing.T) {
	if !testMD(t, "several").IsQuantifier() {
		t.Fatal("not quantifier")
	}
}
func TestMutableFewIsQuantifier(t *testing.T) {
	if !testMD(t, "few").IsQuantifier() {
		t.Fatal("not quantifier")
	}
}
func TestMutableFewerIsQuantifier(t *testing.T) {
	if !testMD(t, "fewer").IsQuantifier() {
		t.Fatal("not quantifier")
	}
}
func TestMutableThanIsConjunction(t *testing.T) {
	if !testMD(t, "than").IsConjunction() || !testMD(t, "Than").IsConjunction() {
		t.Fatal("not conjunction")
	}
}
func TestMutableHerselfIsPronoun(t *testing.T) {
	if !testMD(t, "herself").IsPronoun() || !testMD(t, "Herself").IsPronoun() {
		t.Fatal("not pronoun")
	}
}
func TestMutableDiscussion171(t *testing.T) { checkContains(t, Curated(), "natively") }
func TestMutableImIsCommon(t *testing.T) {
	if !testMD(t, "I'm").Common {
		t.Fatal("not common")
	}
}
func TestMutableFuzzyResultSortedByEditDistance(t *testing.T) {
	assertSortedFuzzy(t, Curated().MutableDictionary)
}
func TestMutableThereIsNotAPronoun(t *testing.T) {
	m := testMD(t, "there")
	if m.IsNominal() || m.IsPronoun() {
		t.Fatal("unexpected nominal")
	}
}
func TestMutableExpandedContainsGiants(t *testing.T)     { checkContains(t, Curated(), "giants") }
func TestMutableExpandedContainsDeallocate(t *testing.T) { checkContains(t, Curated(), "deallocate") }
func TestMutableCuratedContainsRepo(t *testing.T) {
	checkContains(t, Curated(), "repo", "repos", "repo's")
}
func TestMutableCuratedContainsPossessiveAbandonment(t *testing.T) {
	if !testMD(t, "abandonment's").IsPossessiveNoun() {
		t.Fatal("not possessive")
	}
}
func TestMutableContainsInexperienced(t *testing.T) { checkContains(t, Curated(), "inexperienced") }
func TestMutableHasIsNotANominal(t *testing.T) {
	m := Curated().GetWordMetadataStr("has")
	if m == nil || m.IsNominal() {
		t.Fatal("has metadata")
	}
}
func TestMutableIsIsLinkingVerb(t *testing.T) {
	m := Curated().GetWordMetadataStr("is")
	if m == nil || !m.IsLinkingVerb() {
		t.Fatal("is metadata")
	}
}
func TestMutableAreMergedAttrsSameAsSpreadAttrs(t *testing.T) {
	a, e := FromRuneFiles("1\nblork/DGS", AnnotationsStr)
	if e != nil {
		t.Fatal(e)
	}
	b, e := FromRuneFiles("2\nblork/DG\nblork/S", AnnotationsStr)
	if e != nil {
		t.Fatal(e)
	}
	if a.WordCount() != b.WordCount() {
		t.Fatal("size mismatch")
	}
	for _, w := range a.WordsIter() {
		am, bm := a.GetWordMetadataExact(w), b.GetWordMetadataExact(w)
		if bm == nil || !am.Equal(*bm) {
			t.Fatalf("entry mismatch %q", string(w))
		}
	}
}
func TestMutableApartIsNotNoun(t *testing.T) {
	if testMD(t, "apart").IsNoun() {
		t.Fatal("apart noun")
	}
}
func TestMutableBeIsVerbLemma(t *testing.T) {
	m := Curated().GetWordMetadataStr("be")
	if m == nil || !m.IsVerbLemma() {
		t.Fatal("be lemma")
	}
}
func TestMutableTrilemmaInCuratedDictionary(t *testing.T) {
	checkContains(t, Curated(), "trilemma", "trilemmas")
}
func TestMutableGetsPrefixesAsExpected(t *testing.T) {
	d := NewMutableDictionary()
	d.AppendWordStr("predict", DictWordMetadata{})
	d.AppendWordStr("prelude", DictWordMetadata{})
	d.AppendWordStr("preview", DictWordMetadata{})
	d.AppendWordStr("dwight", DictWordMetadata{})
	dict := d
	r := dict.FindWordsWithPrefix([]rune("pre"))
	if len(r) != 3 {
		t.Fatal(r)
	}
	if !slices.ContainsFunc(r, func(v []rune) bool { return string(v) == "predict" }) {
		t.Fatal("missing prefix")
	}
	if !slices.ContainsFunc(r, func(v []rune) bool { return string(v) == "prelude" }) {
		t.Fatal("missing prefix")
	}
	if !slices.ContainsFunc(r, func(v []rune) bool { return string(v) == "preview" }) {
		t.Fatal("missing prefix")
	}
}
func TestMutableGetsCommonPrefixesAsExpected(t *testing.T) {
	d := NewMutableDictionary()
	d.AppendWordStr("pre", DictWordMetadata{})
	d.AppendWordStr("prep", DictWordMetadata{})
	d.AppendWordStr("dwight", DictWordMetadata{})
	dict := d
	r := dict.FindWordsWithCommonPrefix([]rune("preposition"))
	if len(r) != 2 {
		t.Fatal(r)
	}
	if !slices.ContainsFunc(r, func(v []rune) bool { return string(v) == "pre" }) {
		t.Fatal("missing prefix")
	}
	if !slices.ContainsFunc(r, func(v []rune) bool { return string(v) == "prep" }) {
		t.Fatal("missing prefix")
	}
}
func TestTrieGetsPrefixesAsExpected(t *testing.T) {
	d := NewMutableDictionary()
	d.AppendWordStr("predict", DictWordMetadata{})
	d.AppendWordStr("prelude", DictWordMetadata{})
	d.AppendWordStr("preview", DictWordMetadata{})
	d.AppendWordStr("dwight", DictWordMetadata{})
	dict := NewTrieDictionary(d)
	r := dict.FindWordsWithPrefix([]rune("pre"))
	if len(r) != 3 {
		t.Fatal(r)
	}
	if !slices.ContainsFunc(r, func(v []rune) bool { return string(v) == "predict" }) {
		t.Fatal("missing prefix")
	}
	if !slices.ContainsFunc(r, func(v []rune) bool { return string(v) == "prelude" }) {
		t.Fatal("missing prefix")
	}
	if !slices.ContainsFunc(r, func(v []rune) bool { return string(v) == "preview" }) {
		t.Fatal("missing prefix")
	}
}
func TestTrieGetsCommonPrefixesAsExpected(t *testing.T) {
	d := NewMutableDictionary()
	d.AppendWordStr("pre", DictWordMetadata{})
	d.AppendWordStr("prep", DictWordMetadata{})
	d.AppendWordStr("dwight", DictWordMetadata{})
	dict := NewTrieDictionary(d)
	r := dict.FindWordsWithCommonPrefix([]rune("preposition"))
	if len(r) != 2 {
		t.Fatal(r)
	}
	if !slices.ContainsFunc(r, func(v []rune) bool { return string(v) == "pre" }) {
		t.Fatal("missing prefix")
	}
	if !slices.ContainsFunc(r, func(v []rune) bool { return string(v) == "prep" }) {
		t.Fatal("missing prefix")
	}
}
func testFuzzyDictionaries() (*MutableDictionary, *FstDictionary) {
	d := NewMutableDictionary()
	for _, w := range []string{"spelling", "spilling", "selling", "smelling", "shelling"} {
		d.AppendWordStr(w, DictWordMetadata{})
	}
	return d, FstDictionaryFromMutable(d)
}

type fuzzyPair struct {
	Word     string
	Distance uint8
}

func fuzzyPairs(d Dictionary, w string) []fuzzyPair {
	var out []fuzzyPair
	for _, v := range d.FuzzyMatchStr(w, 3, 10) {
		out = append(out, fuzzyPair{string(v.Word), v.EditDistance})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Distance != out[j].Distance {
			return out[i].Distance < out[j].Distance
		}
		return out[i].Word < out[j].Word
	})
	return out
}
func TestFstDamerauTranspositionCostsOne(t *testing.T) {
	d := NewFstDictionary([]WordWithMetadata{{Word: []rune("wofo")}})
	r := d.FuzzyMatchStr("woof", 1, 10)
	if len(r) != 1 || r[0].EditDistance != 1 {
		t.Fatal(r)
	}
}
func TestFstDamerauTranspositionCostsTwo(t *testing.T) {
	var a, b []int
	if levenshtein([]rune("woof"), []rune("wofo"), &a, &b) != 2 {
		t.Fatal("expected non-transposing distance 2")
	}
}
func TestFstFstMapContainsAllInMutableDict(t *testing.T) {
	for _, w := range Curated().WordsIter() {
		if len(normalized(w)) == 0 || !Curated().ContainsExactWord(normalized(w)) {
			t.Fatalf("missing %q", string(w))
		}
	}
}
func TestFstFstContainsHello(t *testing.T) {
	if !Curated().ContainsWord([]rune("hello")) || !Curated().ContainsExactWord([]rune("hello")) {
		t.Fatal("hello missing")
	}
}
func TestFstOnIsNotNominal(t *testing.T) {
	if testMD(t, "on").IsNominal() {
		t.Fatal("on nominal")
	}
}
func TestFstDickensIsNotASwear1656(t *testing.T) {
	m := testMD(t, "Dickens")
	if !m.IsProperNoun() || m.IsSwear() || !m.DerivedFrom.IsNone() || !Curated().ContainsExactWordStr("dickens") || !testMD(t, "dick").IsSwear() {
		t.Fatal("Dickens metadata")
	}
}
func TestFstFuzzyResultSortedByEditDistance(t *testing.T) { assertSortedFuzzy(t, Curated()) }
func TestFstContractionsNotDerived(t *testing.T) {
	for _, w := range []string{"there's", "we're", "here's"} {
		if !testMD(t, w).DerivedFrom.IsNone() {
			t.Fatal(w)
		}
	}
}
func TestFstPluralLlamasDerivedFromLlama(t *testing.T) {
	if !testMD(t, "llamas").DerivedFrom.Contains(NewCanonicalWordIdFromWordStr("llama")) {
		t.Fatal("missing derivation")
	}
}
func TestFstPluralCatsDerivedFromCat(t *testing.T) {
	if !testMD(t, "cats").DerivedFrom.Contains(NewCanonicalWordIdFromWordStr("cat")) {
		t.Fatal("missing derivation")
	}
}
func TestFstUnhappyDerivedFromHappy(t *testing.T) {
	if !testMD(t, "unhappy").DerivedFrom.Contains(NewCanonicalWordIdFromWordStr("happy")) {
		t.Fatal("missing derivation")
	}
}
func TestFstQuicklyDerivedFromQuick(t *testing.T) {
	if !testMD(t, "quickly").DerivedFrom.Contains(NewCanonicalWordIdFromWordStr("quick")) {
		t.Fatal("missing derivation")
	}
}
func TestFstLowercaseFuzzyMatchMatchesMutableDictionary(t *testing.T) {
	m, f := testFuzzyDictionaries()
	a, b := fuzzyPairs(m, "speling"), fuzzyPairs(f, "speling")
	if !reflect.DeepEqual(a, b) || len(b) == 0 || b[0] != (fuzzyPair{"spelling", 1}) {
		t.Fatalf("%v != %v", a, b)
	}
}
func TestFstCapitalizedFuzzyMatchMatchesMutableDictionary(t *testing.T) {
	m, f := testFuzzyDictionaries()
	a, b := fuzzyPairs(m, "Speling"), fuzzyPairs(f, "Speling")
	if !reflect.DeepEqual(a, b) || len(b) == 0 || b[0] != (fuzzyPair{"spelling", 1}) {
		t.Fatalf("%v != %v", a, b)
	}
}
func TestFstUppercaseFuzzyMatchMatchesMutableDictionary(t *testing.T) {
	m, f := testFuzzyDictionaries()
	a, b := fuzzyPairs(m, "SPELING"), fuzzyPairs(f, "SPELING")
	if !reflect.DeepEqual(a, b) || len(b) == 0 || b[0] != (fuzzyPair{"spelling", 1}) {
		t.Fatalf("%v != %v", a, b)
	}
}
func TestFstQueryCasingProducesTheSameFuzzyMatches(t *testing.T) {
	_, f := testFuzzyDictionaries()
	a := fuzzyPairs(f, "speling")
	if !reflect.DeepEqual(a, fuzzyPairs(f, "Speling")) || !reflect.DeepEqual(a, fuzzyPairs(f, "SPELING")) {
		t.Fatal("casing mismatch")
	}
}
func TestMergedContainsExactWordStrIsCaseSensitive(t *testing.T) {
	d := NewMutableDictionary()
	d.AppendWordStr("Foo", DictWordMetadata{})
	m := NewMergedDictionary()
	m.AddDictionary(d)
	if !m.ContainsWordStr("Foo") || !m.ContainsWordStr("foo") || !m.ContainsExactWord([]rune("Foo")) || m.ContainsExactWord([]rune("foo")) || !m.ContainsExactWordStr("Foo") || m.ContainsExactWordStr("foo") {
		t.Fatal("merged casing mismatch")
	}
}
func TestFstMergesCapitalizationsInCanonicalOrder(t *testing.T) {
	d := NewMutableDictionary()
	d.AppendWordStr("word", DictWordMetadata{Noun: &NounData{IsProper: new(false)}})
	d.AppendWordStr("Word", DictWordMetadata{Noun: &NounData{IsProper: new(true)}})
	if d.GetWordMetadataStr("WORD").IsProperNoun() {
		t.Fatal("mutable insertion order changed")
	}
	f := FstDictionaryFromMutable(d)
	if !f.GetWordMetadataStr("WORD").IsProperNoun() {
		t.Fatal("FST did not merge in canonical spelling order")
	}
	words := f.WordsIter()
	if string(words[0]) != "Word" || string(words[1]) != "word" {
		t.Fatal(words)
	}
	variants := f.GetCorrectCapitalizationsOf([]rune("word"))
	if !reflect.DeepEqual(words, variants) {
		t.Fatal(variants)
	}
}
