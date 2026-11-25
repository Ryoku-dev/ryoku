package doc

import (
	"math/rand/v2"
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/text"
	"slices"
	"testing"
)

func TestDetectsSpanish(t *testing.T) {
	{
		src := "Esto es español. Harper no debería marcarlo como inglés."
		dictionary := dict.Curated()
		got := IsDocLikelyEnglish(NewPlainEnglish(src, dictionary), dictionary)
		if got != false {
			t.Fatal(src, got)
		}
	}
}
func TestDetectsFrench(t *testing.T) {
	{
		src := "C'est du français. Il ne devrait pas être marqué comme anglais par Harper."
		dictionary := dict.Curated()
		got := IsDocLikelyEnglish(NewPlainEnglish(src, dictionary), dictionary)
		if got != false {
			t.Fatal(src, got)
		}
	}
}
func TestDetectsShebang(t *testing.T) {
	{
		src := "#! /bin/bash"
		dictionary := dict.Curated()
		got := IsDocLikelyEnglish(NewPlainEnglish(src, dictionary), dictionary)
		if got != false {
			t.Fatal(src, got)
		}
	}
	{
		src := "#! /usr/bin/fish"
		dictionary := dict.Curated()
		got := IsDocLikelyEnglish(NewPlainEnglish(src, dictionary), dictionary)
		if got != false {
			t.Fatal(src, got)
		}
	}
}
func TestDetectsShortEnglish(t *testing.T) {
	{
		src := "This is English!"
		dictionary := dict.Curated()
		got := IsDocLikelyEnglish(NewPlainEnglish(src, dictionary), dictionary)
		if got != true {
			t.Fatal(src, got)
		}
	}
}
func TestDetectsEnglish(t *testing.T) {
	{
		src := "This is perfectly valid English, evn if it has a cople typos."
		dictionary := dict.Curated()
		got := IsDocLikelyEnglish(NewPlainEnglish(src, dictionary), dictionary)
		if got != true {
			t.Fatal(src, got)
		}
	}
}
func TestDetectsExpressiveEnglish(t *testing.T) {
	{
		src := "Look above! That is real English! So is this: bippity bop!"
		dictionary := dict.Curated()
		got := IsDocLikelyEnglish(NewPlainEnglish(src, dictionary), dictionary)
		if got != true {
			t.Fatal(src, got)
		}
	}
}
func TestDetectsPythonFib(t *testing.T) {
	{
		src := "\ndef fibIter(n):\n    if n < 2:\n        return n\n    fibPrev = 1\n    fib = 1\n    for _ in range(2, n):\n        fibPrev, fib = fib, fib + fibPrev\n    return fib\n        "
		dictionary := dict.Curated()
		got := IsDocLikelyEnglish(NewPlainEnglish(src, dictionary), dictionary)
		if got != false {
			t.Fatal(src, got)
		}
	}
}
func TestMixedFrenchEnglishPark(t *testing.T) {
	{
		src := "Je voudrais promener au the park a huit heures with ma voisine"
		dictionary := dict.Curated()
		got := IsDocLikelyEnglish(NewPlainEnglish(src, dictionary), dictionary)
		if got != false {
			t.Fatal(src, got)
		}
	}
}
func TestMixedFrenchEnglishDrunk(t *testing.T) {
	{
		src := "Je ne suis pas drunk, je suis only ivre by you"
		dictionary := dict.Curated()
		got := IsDocLikelyEnglish(NewPlainEnglish(src, dictionary), dictionary)
		if got != false {
			t.Fatal(src, got)
		}
	}
}
func TestMixedFrenchEnglishDress(t *testing.T) {
	{
		src := "Je buy une robe nouveau chaque Tuesday, mais aujourd'hui, je don't have temps"
		dictionary := dict.Curated()
		got := IsDocLikelyEnglish(NewPlainEnglish(src, dictionary), dictionary)
		if got != false {
			t.Fatal(src, got)
		}
	}
}
func TestEnglishMotto(t *testing.T) {
	{
		src := "I have a simple motto in life"
		dictionary := dict.Curated()
		got := IsDocLikelyEnglish(NewPlainEnglish(src, dictionary), dictionary)
		if got != true {
			t.Fatal(src, got)
		}
	}
}
func TestCarIsSingularNoun(t *testing.T) {
	d := NewPlainEnglishCurated("car")
	k := d.tokens[0].Kind
	if !k.IsSingularNoun() {
		t.Fatal(k)
	}
}
func TestTrafficIsMassNounOnly(t *testing.T) {
	d := NewPlainEnglishCurated("traffic")
	k := d.tokens[0].Kind
	if !k.IsMassNounOnly() {
		t.Fatal(k)
	}
}
func TestEquipmentIsMassNoun(t *testing.T) {
	d := NewPlainEnglishCurated("equipment")
	k := d.tokens[0].Kind
	if !k.IsMassNoun() {
		t.Fatal(k)
	}
}
func TestEquipmentIsNonCountableNoun(t *testing.T) {
	d := NewPlainEnglishCurated("equipment")
	k := d.tokens[0].Kind
	if !k.IsNonCountableNoun() {
		t.Fatal(k)
	}
}
func TestEquipmentIsntCountableNoun(t *testing.T) {
	d := NewPlainEnglishCurated("equipment")
	k := d.tokens[0].Kind
	if k.IsCountableNoun() {
		t.Fatal(k)
	}
}
func TestAteIsSimplePastOnly(t *testing.T) {
	d := NewPlainEnglishCurated("ate")
	k := d.tokens[0].Kind
	if !k.IsVerbSimplePastOnly() {
		t.Fatal(k)
	}
	if k.IsVerbPastParticipleOnly() {
		t.Fatal(k)
	}
}
func TestEatenIsPastParticipleOnly(t *testing.T) {
	d := NewPlainEnglishCurated("eaten")
	k := d.tokens[0].Kind
	if !k.IsVerbPastParticipleOnly() {
		t.Fatal(k)
	}
	if k.IsVerbSimplePastOnly() {
		t.Fatal(k)
	}
}
func TestThoughtIsRegularPastForm(t *testing.T) {
	d := NewPlainEnglishCurated("thought")
	k := d.tokens[0].Kind
	if !k.IsVerbPreteriteAndParticipleForm() {
		t.Fatal(k)
	}
}
func TestOovWordIsOov(t *testing.T) {
	d := NewPlainEnglishCurated("nonexistentword")
	k := d.tokens[0].Kind
	if !k.IsOov() {
		t.Fatal(k)
	}
}
func TestKnownWordIsNotOov(t *testing.T) {
	d := NewPlainEnglishCurated("car")
	k := d.tokens[0].Kind
	if k.IsOov() {
		t.Fatal(k)
	}
}
func TestNonWordTokensAreNotOov(t *testing.T) {
	d := NewPlainEnglishCurated("Hello, world!")
	if d.tokens[1].Kind.IsOov() {
		t.Fatal(d.tokens[1].Kind)
	}
	if d.tokens[3].Kind.IsOov() {
		t.Fatal(d.tokens[3].Kind)
	}
}

func TestSingleLetter(t *testing.T) {
	tokens := PlainEnglish{}.ParseStr("a")
	if len(tokens) != 1 || !tokens[0].Kind.IsWord() {
		t.Fatal(tokens)
	}
}
func TestSentence(t *testing.T) {
	tokens := PlainEnglish{}.ParseStr("hello world, my friend")
	want := []TokenKindTag{Word, Space, Word, Punctuation, Space, Word, Space, Word}
	if len(tokens) != len(want) {
		t.Fatal(tokens)
	}
	for i, k := range want {
		if tokens[i].Kind.Kind != k {
			t.Fatal(tokens)
		}
	}
	if !tokens[3].Kind.IsComma() {
		t.Fatal(tokens)
	}
}
func TestParsesNonEnglish(t *testing.T) {
	for _, src := range []string{"Løvetann", "Naïve"} {
		tokens := PlainEnglish{}.ParseStr(src)
		if len(tokens) != 1 || !tokens[0].Kind.IsWord() {
			t.Fatal(tokens)
		}
	}
}
func TestBumpsExisting(t *testing.T) {
	m := NewBlankMask()
	m.PushAllowed(text.NewSpanWithLen(0, 1))
	m.PushAllowed(text.NewSpanWithLen(1, 2))
	if len(m.allowed) != 1 {
		t.Fatal(m)
	}
}
func TestMergesWhitespaceSep(t *testing.T) {
	m := NewBlankMask()
	for _, start := range []int{0, 5, 10} {
		m.PushAllowed(text.NewSpanWithLen(start, 4))
	}
	if len(m.allowed) != 3 {
		t.Fatal(m)
	}
	m.MergeWhitespaceSep([]rune("word word\nword"))
	if len(m.allowed) != 1 {
		t.Fatal(m)
	}
}
func allowedStrings(m TextMask, src []rune) []string {
	var out []string
	for _, a := range m.IterAllowed(src) {
		out = append(out, string(a.Content))
	}
	return out
}
func TestIncludeMatches(t *testing.T) {
	src := []rune("foo [ignore] bar [drop]")
	m, ok := NewRegexMasker(`\[[^\]]+\]`, false)
	if !ok {
		t.Fatal("regex")
	}
	if got := allowedStrings(m.CreateMask(src), src); !slices.Equal(got, []string{"[ignore]", "[drop]"}) {
		t.Fatal(got)
	}
}
func TestExcludeMatches(t *testing.T) {
	src := []rune("foo [ignore] bar [drop]")
	m, ok := NewRegexMasker(`\[[^\]]+\]`, true)
	if !ok {
		t.Fatal("regex")
	}
	if got := allowedStrings(m.CreateMask(src), src); !slices.Equal(got, []string{"foo ", " bar "}) {
		t.Fatal(got)
	}
}
func TestUnicodeOffsetsAreConvertedToCharSpans(t *testing.T) {
	src := []rune("A🙂B🙂C")
	m, _ := NewRegexMasker("🙂B🙂", false)
	if got := allowedStrings(m.CreateMask(src), src); !slices.Equal(got, []string{"🙂B🙂"}) {
		t.Fatal(got)
	}
}
func TestCanMatchEverything(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	m, _ := NewRegexMasker(".*", false)
	for range 1000 {
		src := make([]rune, r.IntN(100)+1)
		for i := range src {
			src[i] = rune(r.IntN(95) + 32)
		}
		mask := m.CreateMask(src)
		if len(mask.allowed) != 1 || mask.allowed[0] != (text.Span{Start: 0, End: len(src)}) {
			t.Fatal(mask)
		}
	}
}
func TestCommaSeparatesChunks(t *testing.T) {
	chunkStrings(t, "foo bar, baz", []string{"foo bar,", " baz"})
}
func TestSemicolonSeparatesChunks(t *testing.T) {
	chunkStrings(t, "foo bar; baz", []string{"foo bar;", " baz"})
}
func chunkStrings(t *testing.T, src string, want []string) {
	d := NewBasicTokenize(src, PlainEnglish{})
	chunks := d.IterChunks()
	if len(chunks) != len(want) {
		t.Fatal(chunks)
	}
	for i, c := range chunks {
		got, _ := GetStr(c, d.source)
		if got != want[i] {
			t.Fatal(got)
		}
	}
}
func TestMatchesKebab(t *testing.T) {
	tokens := PlainEnglish{}.ParseStr("kebab-case")
	if len(tokens) != 3 || !tokens[0].Kind.IsWord() || !tokens[1].Kind.IsCaseSeparator() || !tokens[2].Kind.IsWord() {
		t.Fatal(tokens)
	}
}
func TestNoCollapse(t *testing.T) {
	p := NewCollapseIdentifiers(PlainEnglish{}, dict.Curated())
	if toks := p.ParseStr("This is a test."); len(toks) != 8 {
		t.Fatal(toks)
	}
}
func assertCollapse(t *testing.T, source string, before, after int, words ...string) {
	t.Helper()
	curated := dict.Curated()
	p := NewCollapseIdentifiers(PlainEnglish{}, curated)
	if toks := p.ParseStr(source); len(toks) != before {
		t.Fatal(toks)
	}
	custom := dict.NewMutableDictionary()
	for _, w := range words {
		custom.AppendWordStr(w, dict.DictWordMetadata{})
	}
	merged := dict.NewMergedDictionary()
	merged.AddDictionary(curated)
	merged.AddDictionary(custom)
	p = NewCollapseIdentifiers(PlainEnglish{}, merged)
	if toks := p.ParseStr(source); len(toks) != after {
		t.Fatal(len(toks), toks)
	}
}
func TestOneCollapse(t *testing.T) {
	assertCollapse(t, "This is a separated_identifier, wow!", 13, 11, "separated_identifier")
}
func TestKebabCollapse(t *testing.T) {
	assertCollapse(t, "This is a separated-identifier, wow!", 13, 11, "separated-identifier")
}
func TestDoubleCollapse(t *testing.T) {
	assertCollapse(t, "This is a separated_identifier_token, wow!", 15, 11, "separated_identifier_token")
}
func TestTwoCollapses(t *testing.T) {
	assertCollapse(t, "This is a separated_identifier, wow! separated_identifier", 17, 13, "separated_identifier")
}
func TestOverlappingIdentifiers(t *testing.T) {
	assertCollapse(t, "This is a separated_identifier_token, wow!", 15, 15, "separated_identifier", "identifier_token")
}
func TestNestedIdentifiers(t *testing.T) {
	assertCollapse(t, "This is a separated_identifier_token, wow!", 15, 11, "separated_identifier_token", "separated_identifier")
}
func TestMixedSpanishEnglishBreakfast(t *testing.T) {
	dictionary := dict.Curated()
	d := NewDocument("En la mañana, como a dish de los huevos, un poquito of tocino, y a lot of leche.", NewIsolateEnglish(PlainEnglish{}, dictionary), dictionary)
	if len(d.IterWords()) != 0 || len(d.IterPunctuations()) != 0 {
		t.Fatal(d.FatStringTokens())
	}
}
func TestMixedSpanishEnglishPolitics(t *testing.T) {
	dictionary := dict.Curated()
	d := NewDocument("No estoy of acuerdo con the politics de Los estados unidos ahora; pienso que we need mas diversidad in el gobierno.", NewIsolateEnglish(PlainEnglish{}, dictionary), dictionary)
	if len(d.IterWords()) != 0 || len(d.IterPunctuations()) != 0 {
		t.Fatal(d.FatStringTokens())
	}
}
func TestEnglishNoEditMotto(t *testing.T) {
	dictionary := dict.Curated()
	d := NewDocument("I have a simple motto in life: ", NewIsolateEnglish(PlainEnglish{}, dictionary), dictionary)
	if got := d.String(); got != "I have a simple motto in life: " {
		t.Fatal(got)
	}
}
func TestChunkedTradChineseEnglish(t *testing.T) {
	dictionary := dict.Curated()
	d := NewDocument("I have a simple motto in life: 如果你渴了，就喝水。", NewIsolateEnglish(PlainEnglish{}, dictionary), dictionary)
	if got := d.String(); got != "I have a simple motto in life:" {
		t.Fatal(got)
	}
}
func TestChunkedTradPolishEnglish(t *testing.T) {
	dictionary := dict.Curated()
	d := NewDocument("I have a simple motto in life: jeśli jesteś spragniony, napij się wody.", NewIsolateEnglish(PlainEnglish{}, dictionary), dictionary)
	if got := d.String(); got != "I have a simple motto in life:" {
		t.Fatal(got)
	}
}
