package doc

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/text"
	"strings"
	"testing"
)

func assertTokenCount(t *testing.T, source string, count int) {
	t.Helper()
	d := NewPlainEnglishCurated(source)
	if len(d.tokens) != count {
		t.Fatalf("%q: got %d want %d: %#v", source, len(d.tokens), count, d.FatStringTokens())
	}
}
func TestSimpleContraction(t *testing.T)  { assertTokenCount(t, "isn't", 1) }
func TestSimpleContraction2(t *testing.T) { assertTokenCount(t, "wasn't", 1) }
func TestSimpleContraction3(t *testing.T) { assertTokenCount(t, "There's", 1) }
func TestSimpleContraction4(t *testing.T) { assertTokenCount(t, "doesn't", 1) }
func TestMediumContraction(t *testing.T)  { assertTokenCount(t, "isn't wasn't", 3) }
func TestMediumContraction2(t *testing.T) { assertTokenCount(t, "There's no way", 5) }
func TestCondensesNumberSuffixes(t *testing.T) {
	assertTokenCount(t, "1st", 1)
	assertTokenCount(t, "This is the 2nd test", 9)
	assertTokenCount(t, "This is the 3rd test", 9)
	assertTokenCount(t, "It works even with weird capitalization like this: 600nD", 18)
}
func TestCondensesIe(t *testing.T) {
	assertTokenCount(t, "There is a thing (i.e. that one)", 15)
	assertTokenCount(t, "We are trying to condense \"i.e.\"", 13)
	assertTokenCount(t, "Condenses words like \"i.e.\", \"e.g.\" and \"N.S.A.\"", 20)
}
func TestCondensesEg(t *testing.T) {
	assertTokenCount(t, "We are trying to condense \"e.g.\"", 13)
	assertTokenCount(t, "Condenses words like \"i.e.\", \"e.g.\" and \"N.S.A.\"", 20)
}
func TestCondensesNsa(t *testing.T) {
	assertTokenCount(t, "Condenses words like \"i.e.\", \"e.g.\" and \"N.S.A.\"", 20)
}
func TestParsesEllipsis(t *testing.T)      { assertTokenCount(t, "...", 1) }
func TestParsesLongEllipsis(t *testing.T)  { assertTokenCount(t, ".....", 1) }
func TestParsesShortEllipsis(t *testing.T) { assertTokenCount(t, "..", 1) }
func TestSelectsTokenAtOffset(t *testing.T) {
	d := NewPlainEnglishCurated("Foo bar baz")
	tok, ok := d.GetTokenOffset(1, -1)
	if !ok || tok.Span != (text.Span{Start: 0, End: 3}) {
		t.Fatal(tok)
	}
}
func TestCantSelectTokenBeforeStart(t *testing.T) {
	d := NewPlainEnglishCurated("Foo bar baz")
	tok, ok := d.GetTokenOffset(0, -1)
	if ok {
		t.Fatal(tok)
	}
}
func TestSelectNextWordPosOffset(t *testing.T) {
	d := NewPlainEnglishCurated("Foo bar baz")
	tok, ok := d.GetNextWordFromOffset(0, 1)
	if !ok || tok.GetStr(d.source) != "bar" {
		t.Fatal(tok)
	}
}
func TestSelectNextWordNegOffset(t *testing.T) {
	d := NewPlainEnglishCurated("Foo bar baz")
	tok, ok := d.GetNextWordFromOffset(2, -1)
	if !ok || tok.GetStr(d.source) != "Foo" {
		t.Fatal(tok)
	}
}
func TestCantSelectNextWordNotFromWhitespace(t *testing.T) {
	d := NewPlainEnglishCurated("Foo bar baz")
	tok, ok := d.GetNextWordFromOffset(0, 2)
	if ok {
		t.Fatal(tok)
	}
}
func TestCantSelectNextWordBeforeStart(t *testing.T) {
	d := NewPlainEnglishCurated("Foo bar baz")
	tok, ok := d.GetNextWordFromOffset(0, -1)
	if ok {
		t.Fatal(tok)
	}
}
func TestCantSelectNextWordWithPunctuationInsteadOfWhitespace(t *testing.T) {
	d := NewPlainEnglishCurated("Foo, bar, baz")
	tok, ok := d.GetNextWordFromOffset(0, 1)
	if ok {
		t.Fatal(tok)
	}
}
func TestCantSelectNextWordWithPunctuationAfterWhitespace(t *testing.T) {
	d := NewPlainEnglishCurated("Foo \"bar\", baz")
	tok, ok := d.GetNextWordFromOffset(0, 1)
	if ok {
		t.Fatal(tok)
	}
}
func TestCondensesFilenameExtensions(t *testing.T) {
	d := NewPlainEnglishCurated(".c and .exe and .js")
	if !d.tokens[0].Kind.IsUnlintable() {
		t.Fatal(d.FatStringTokens())
	}
	if !d.tokens[4].Kind.IsUnlintable() {
		t.Fatal(d.FatStringTokens())
	}
	if !d.tokens[8].Kind.IsUnlintable() {
		t.Fatal(d.FatStringTokens())
	}
}
func TestCondenseFilenameExtensionOkAtStartAndEnd(t *testing.T) {
	d := NewPlainEnglishCurated(".c and .EXE")
	if !(len(d.tokens) == 5) {
		t.Fatal(len(d.tokens), d.FatStringTokens())
	}
	if !d.tokens[0].Kind.IsUnlintable() {
		t.Fatal(d.FatStringTokens())
	}
	if !d.tokens[4].Kind.IsUnlintable() {
		t.Fatal(d.FatStringTokens())
	}
}
func TestDoesntCondenseFilenameExtensionsWithMixedCase(t *testing.T) {
	d := NewPlainEnglishCurated(".c and .Exe")
	if !(len(d.tokens) == 6) {
		t.Fatal(len(d.tokens), d.FatStringTokens())
	}
	if !d.tokens[0].Kind.IsUnlintable() {
		t.Fatal(d.FatStringTokens())
	}
	if !d.tokens[4].Kind.IsPunctuation() {
		t.Fatal(d.FatStringTokens())
	}
	if !d.tokens[5].Kind.IsWord() {
		t.Fatal(d.FatStringTokens())
	}
}
func TestDoesntCondenseFilenameExtensionsWithNonLetters(t *testing.T) {
	d := NewPlainEnglishCurated(".COM and .C0M")
	if !(len(d.tokens) == 6) {
		t.Fatal(len(d.tokens), d.FatStringTokens())
	}
	if !d.tokens[0].Kind.IsUnlintable() {
		t.Fatal(d.FatStringTokens())
	}
	if !d.tokens[4].Kind.IsPunctuation() {
		t.Fatal(d.FatStringTokens())
	}
	if !d.tokens[5].Kind.IsWord() {
		t.Fatal(d.FatStringTokens())
	}
}
func TestDoesntCondenseFilenameExtensionsLongerThanThree(t *testing.T) {
	d := NewPlainEnglishCurated(".dll and .dlls")
	if !(len(d.tokens) == 6) {
		t.Fatal(len(d.tokens), d.FatStringTokens())
	}
	if !d.tokens[0].Kind.IsUnlintable() {
		t.Fatal(d.FatStringTokens())
	}
	if !d.tokens[4].Kind.IsPunctuation() {
		t.Fatal(d.FatStringTokens())
	}
	if !d.tokens[5].Kind.IsWord() {
		t.Fatal(d.FatStringTokens())
	}
}
func TestCondenseTldrUppercase(t *testing.T) {
	d := NewPlainEnglishCurated("TL;DR")
	if !(len(d.tokens) == 1) {
		t.Fatal(len(d.tokens), d.FatStringTokens())
	}
	if !d.tokens[0].Kind.IsWord() {
		t.Fatal(d.FatStringTokens())
	}
	if d.tokens[0].Span.Len() != 5 {
		t.Fatal(d.tokens[0])
	}
}
func TestCondenseTldrLowercase(t *testing.T) {
	d := NewPlainEnglishCurated("tl;dr")
	if !(len(d.tokens) == 1) {
		t.Fatal(len(d.tokens), d.FatStringTokens())
	}
	if !d.tokens[0].Kind.IsWord() {
		t.Fatal(d.FatStringTokens())
	}
}
func TestCondenseTldrMixedCase1(t *testing.T) {
	d := NewPlainEnglishCurated("tl;DR")
	if !(len(d.tokens) == 1) {
		t.Fatal(len(d.tokens), d.FatStringTokens())
	}
	if !d.tokens[0].Kind.IsWord() {
		t.Fatal(d.FatStringTokens())
	}
}
func TestCondenseTldrMixedCase2(t *testing.T) {
	d := NewPlainEnglishCurated("TL;Dr")
	if !(len(d.tokens) == 1) {
		t.Fatal(len(d.tokens), d.FatStringTokens())
	}
	if !d.tokens[0].Kind.IsWord() {
		t.Fatal(d.FatStringTokens())
	}
}
func TestCondenseCommonTopLevelDomains(t *testing.T) {
	d := NewPlainEnglishCurated(".blog and .com and .NET")
	if !(len(d.tokens) == 9) {
		t.Fatal(len(d.tokens), d.FatStringTokens())
	}
	if !d.tokens[0].Kind.IsUnlintable() {
		t.Fatal(d.FatStringTokens())
	}
	if !d.tokens[4].Kind.IsUnlintable() {
		t.Fatal(d.FatStringTokens())
	}
	if !d.tokens[8].Kind.IsUnlintable() {
		t.Fatal(d.FatStringTokens())
	}
}
func TestCondenseCommonTopLevelDomainsInParens(t *testing.T) {
	d := NewPlainEnglishCurated("(.blog)")
	if !(len(d.tokens) == 3) {
		t.Fatal(len(d.tokens), d.FatStringTokens())
	}
	if !d.tokens[0].Kind.IsOpenRound() {
		t.Fatal(d.FatStringTokens())
	}
	if !d.tokens[1].Kind.IsUnlintable() {
		t.Fatal(d.FatStringTokens())
	}
	if !d.tokens[2].Kind.IsCloseRound() {
		t.Fatal(d.FatStringTokens())
	}
}
func TestDoesntCondenseUnknownTopLevelDomains(t *testing.T) {
	d := NewPlainEnglishCurated(".harper")
	if !(len(d.tokens) == 2) {
		t.Fatal(len(d.tokens), d.FatStringTokens())
	}
	if !d.tokens[0].Kind.IsPunctuation() {
		t.Fatal(d.FatStringTokens())
	}
	if !d.tokens[1].Kind.IsWord() {
		t.Fatal(d.FatStringTokens())
	}
}
func TestCondenseRAndDCaps(t *testing.T) {
	d := NewPlainEnglishCurated("R&D")
	if !(len(d.tokens) == 1) {
		t.Fatal(len(d.tokens), d.FatStringTokens())
	}
	if !d.tokens[0].Kind.IsWord() {
		t.Fatal(d.FatStringTokens())
	}
}
func TestCondenseRAndDMixedCase(t *testing.T) {
	d := NewPlainEnglishCurated("R&d")
	if !(len(d.tokens) == 1) {
		t.Fatal(len(d.tokens), d.FatStringTokens())
	}
	if !d.tokens[0].Kind.IsWord() {
		t.Fatal(d.FatStringTokens())
	}
}
func TestCondenseRAndDLowercase(t *testing.T) {
	d := NewPlainEnglishCurated("r&d")
	if !(len(d.tokens) == 1) {
		t.Fatal(len(d.tokens), d.FatStringTokens())
	}
	if !d.tokens[0].Kind.IsWord() {
		t.Fatal(d.FatStringTokens())
	}
}
func TestDontCondenseRAndDWithSpaces(t *testing.T) {
	d := NewPlainEnglishCurated("R & D")
	if !(len(d.tokens) == 5) {
		t.Fatal(len(d.tokens), d.FatStringTokens())
	}
	if !d.tokens[0].Kind.IsWord() {
		t.Fatal(d.FatStringTokens())
	}
	if !d.tokens[1].Kind.IsWhitespace() {
		t.Fatal(d.FatStringTokens())
	}
	if !d.tokens[2].Kind.IsAmpersand() {
		t.Fatal(d.FatStringTokens())
	}
	if !d.tokens[3].Kind.IsWhitespace() {
		t.Fatal(d.FatStringTokens())
	}
	if !d.tokens[4].Kind.IsWord() {
		t.Fatal(d.FatStringTokens())
	}
}
func TestCondenseQAndA(t *testing.T) {
	d := NewPlainEnglishCurated("A Q&A platform software for teams at any scales.")
	if !(len(d.tokens) >= 3) {
		t.Fatal(len(d.tokens), d.FatStringTokens())
	}
	if !d.tokens[2].Kind.IsWord() {
		t.Fatal(d.FatStringTokens())
	}
	if d.tokens[2].GetStr(d.source) != "Q&A" {
		t.Fatal(d.tokens[2])
	}
}
func TestDontAllowMixedRAndDWithQAndA(t *testing.T) {
	d := NewPlainEnglishCurated("R&A or Q&D")
	if !(len(d.tokens) == 9) {
		t.Fatal(len(d.tokens), d.FatStringTokens())
	}
	if !d.tokens[1].Kind.IsAmpersand() && !d.tokens[7].Kind.IsAmpersand() {
		t.Fatal(d.tokens)
	}
}
func TestCondenseIo(t *testing.T) {
	d := NewPlainEnglishCurated("I/O")
	if !(len(d.tokens) == 1) {
		t.Fatal(len(d.tokens), d.FatStringTokens())
	}
	if !d.tokens[0].Kind.IsWord() {
		t.Fatal(d.FatStringTokens())
	}
}
func TestSelectsTokenAtCharIndex(t *testing.T) {
	d := NewPlainEnglishCurated("There were three little pigs. They built three little homes.")
	tok, ok := d.GetTokenAtCharIndex(19)
	if !ok || !tok.Kind.IsWord() || tok.Span != (text.Span{Start: 17, End: 23}) {
		t.Fatal(tok)
	}
}
func TestParsesSentencesCorrectly(t *testing.T) {
	src := []rune("There were three little pigs. They built three little homes.")
	tokens := PlainEnglish{}.Parse(src)
	want := []string{"There were three little pigs.", " They built three little homes."}
	sentences := IterSentences(tokens)
	if len(sentences) != len(want) {
		t.Fatal(sentences)
	}
	for i, s := range sentences {
		got, _ := GetStr(s, src)
		if got != want[i] {
			t.Fatal(got)
		}
	}
}
func TestDialectAluminium(t *testing.T) {
	f := MostUsedDialects(NewPlainEnglishCurated("Aluminium was used."))
	if !f.IsDialectEnabledStrict(dict.British) || f.IsDialectEnabledStrict(dict.American) {
		t.Fatal(f)
	}
}
func TestDialectAluminum(t *testing.T) {
	f := MostUsedDialects(NewPlainEnglishCurated("Aluminum was used."))
	if !f.IsDialectEnabledStrict(dict.American) || f.IsDialectEnabledStrict(dict.British) {
		t.Fatal(f)
	}
}
func TestCondenseFilenameExtensionInParens(t *testing.T) {
	d := NewPlainEnglishCurated("true for the manual installation when trying to run the executable(.exe) after a manual download")
	if len(d.tokens) <= 23 || !d.tokens[21].Kind.IsOpenRound() || !d.tokens[22].Kind.IsUnlintable() || !d.tokens[23].Kind.IsCloseRound() {
		t.Fatal(d.FatStringTokens())
	}
}
func TestCondenseTldrPural(t *testing.T) {
	d := NewPlainEnglishCurated("managing the flow between components to produce relevant TL;DRs of current news articles")
	found := 0
	for _, tok := range d.tokens {
		if !tok.Kind.IsWord() && !tok.Kind.IsWhitespace() {
			t.Fatal(tok)
		}
		if strings.ContainsRune(tok.GetStr(d.source), ';') {
			found++
			if tok.GetStr(d.source) != "TL;DRs" {
				t.Fatal(tok)
			}
		}
	}
	if found != 1 {
		t.Fatal(found)
	}
}
