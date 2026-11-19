package doc

import (
	"ryoku-rashin/internal/grammar/text"
	"testing"
)

func TestLexes0(t *testing.T) {
	got, ok := lexNumber([]rune("0"))
	if !ok {
		t.Fatal("no token")
	}
	if got.Token.Kind != Number {
		t.Fatalf("token %#v", got)
	}
}
func TestLexes0Point0(t *testing.T) {
	got, ok := lexNumber([]rune("0.0"))
	if !ok {
		t.Fatal("no token")
	}
	if got.Token.Kind != Number {
		t.Fatalf("token %#v", got)
	}
}
func TestLexes00(t *testing.T) {
	got, ok := lexNumber([]rune("00"))
	if !ok {
		t.Fatal("no token")
	}
	if got.Token.Kind != Number {
		t.Fatalf("token %#v", got)
	}
}
func TestLexesPi(t *testing.T) {
	got, ok := lexNumber([]rune("3.1415926535897932384626433832795028841971693993751058209749445923078164062862089986280348253421170679"))
	if !ok {
		t.Fatal("no token")
	}
	if got.Token.Kind != Number {
		t.Fatalf("token %#v", got)
	}
}
func TestLexesSpeedOfLight(t *testing.T) {
	got, ok := lexNumber([]rune("3.00e8"))
	if !ok {
		t.Fatal("no token")
	}
	if got.Token.Kind != Number {
		t.Fatalf("token %#v", got)
	}
}
func TestDoesntLexCjkNumeral(t *testing.T) {
	got, ok := lexNumber([]rune("二"))
	if ok {
		t.Fatalf("unexpected token %#v", got)
	}
}
func TestDoesntLexThaiDigit(t *testing.T) {
	got, ok := lexNumber([]rune("๑"))
	if ok {
		t.Fatalf("unexpected token %#v", got)
	}
}
func TestLexesCjkAsUnlintable(t *testing.T) {
	got, ok := lexWord([]rune("世"))
	if ok {
		t.Fatalf("unexpected token %#v", got)
	}
}
func TestLexesYoutubeAsHostname(t *testing.T) {
	got := LexEnglishToken([]rune("youtube.com"))
	if got.Token.Kind != Hostname {
		t.Fatalf("token %#v", got)
	}
	if got.NextIndex != len([]rune("youtube.com")) {
		t.Fatal(got)
	}
}
func TestDoesntLexRegexMiniRange(t *testing.T) {
	got := LexEnglishToken([]rune("[]"))
	if got.Token.Kind == Regexish {
		t.Fatalf("token %#v", got)
	}
}
func TestLexesRegexOneLetter(t *testing.T) {
	got := LexEnglishToken([]rune("[a]"))
	if got.Token.Kind != Regexish {
		t.Fatalf("token %#v", got)
	}
	if got.NextIndex != 3 {
		t.Fatal(got)
	}
}
func TestLexesRegexTwoLetters(t *testing.T) {
	got := LexEnglishToken([]rune("[az]"))
	if got.Token.Kind != Regexish {
		t.Fatalf("token %#v", got)
	}
	if got.NextIndex != 4 {
		t.Fatal(got)
	}
}
func TestLexesRegexDigits(t *testing.T) {
	got := LexEnglishToken([]rune("[123]"))
	if got.Token.Kind != Regexish {
		t.Fatalf("token %#v", got)
	}
	if got.NextIndex != 5 {
		t.Fatal(got)
	}
}
func TestLexesRegexTwoAlphanumeric(t *testing.T) {
	got := LexEnglishToken([]rune("[a0b1c2]"))
	if got.Token.Kind != Regexish {
		t.Fatalf("token %#v", got)
	}
	if got.NextIndex != 8 {
		t.Fatal(got)
	}
}
func TestLexesRegexOneRange(t *testing.T) {
	got := LexEnglishToken([]rune("[a-z]"))
	if got.Token.Kind != Regexish {
		t.Fatalf("token %#v", got)
	}
	if got.NextIndex != 5 {
		t.Fatal(got)
	}
}
func TestLexesRegexLetterPlusRange(t *testing.T) {
	got := LexEnglishToken([]rune("[ax-z]"))
	if got.Token.Kind != Regexish {
		t.Fatalf("token %#v", got)
	}
	if got.NextIndex != 6 {
		t.Fatal(got)
	}
}
func TestLexesRegexRangePlusLetter(t *testing.T) {
	got := LexEnglishToken([]rune("[a-cz]"))
	if got.Token.Kind != Regexish {
		t.Fatalf("token %#v", got)
	}
	if got.NextIndex != 6 {
		t.Fatal(got)
	}
}
func TestLexesRegexTwoRanges(t *testing.T) {
	got := LexEnglishToken([]rune("[a-cx-z]"))
	if got.Token.Kind != Regexish {
		t.Fatalf("token %#v", got)
	}
	if got.NextIndex != 8 {
		t.Fatal(got)
	}
}
func TestDoesntLexRegexBrokenTwoRanges(t *testing.T) {
	got := LexEnglishToken([]rune("[a-x-z]"))
	if got.Token.Kind != Punctuation {
		t.Fatalf("token %#v", got)
	}
	if got.Token.Punctuation.Kind != text.OpenSquare {
		t.Fatal(got)
	}
	if got.NextIndex != 1 {
		t.Fatal(got)
	}
}
func TestDoesntLexRegexHyphenAtStart(t *testing.T) {
	got := LexEnglishToken([]rune("[a-]"))
	if got.Token.Kind == Regexish {
		t.Fatalf("token %#v", got)
	}
}
func TestDoesntLexRegexHyphenAtEnd(t *testing.T) {
	got := LexEnglishToken([]rune("[-z]"))
	if got.Token.Kind == Regexish {
		t.Fatalf("token %#v", got)
	}
}
func TestLexesGoodHexNumeric(t *testing.T) {
	got, ok := lexHexNumber([]rune("0x0"))
	if !ok {
		t.Fatal("no token")
	}
	if got.Token.Kind != Number {
		t.Fatalf("token %#v", got)
	}
}
func TestLexesGoodHexLowercase(t *testing.T) {
	got, ok := lexHexNumber([]rune("0xa"))
	if !ok {
		t.Fatal("no token")
	}
	if got.Token.Kind != Number {
		t.Fatalf("token %#v", got)
	}
}
func TestLexesGoodHexUppercase(t *testing.T) {
	got, ok := lexHexNumber([]rune("0xF"))
	if !ok {
		t.Fatal("no token")
	}
	if got.Token.Kind != Number {
		t.Fatalf("token %#v", got)
	}
}
func TestLexesGoodHexMixedCase(t *testing.T) {
	got, ok := lexHexNumber([]rune("0xaF"))
	if !ok {
		t.Fatal("no token")
	}
	if got.Token.Kind != Number {
		t.Fatalf("token %#v", got)
	}
}
func TestLexesGoodHexLowercaseLong(t *testing.T) {
	got, ok := lexHexNumber([]rune("0x0123456789abcdef"))
	if !ok {
		t.Fatal("no token")
	}
	if got.Token.Kind != Number {
		t.Fatalf("token %#v", got)
	}
}
func TestLexesGoodHexUppercaseLong(t *testing.T) {
	got, ok := lexHexNumber([]rune("0x0123456789ABCDEF"))
	if !ok {
		t.Fatal("no token")
	}
	if got.Token.Kind != Number {
		t.Fatalf("token %#v", got)
	}
}
func TestDoesNotLexPrefixOnly(t *testing.T) {
	got, ok := lexHexNumber([]rune("0x"))
	if ok {
		t.Fatalf("unexpected token %#v", got)
	}
}
func TestDoesNotLexBadAlphabetic(t *testing.T) {
	got, ok := lexHexNumber([]rune("0xg"))
	if ok {
		t.Fatalf("unexpected token %#v", got)
	}
}
func TestDoesNotLexBadAfterGood(t *testing.T) {
	got, ok := lexHexNumber([]rune("0x123g"))
	if ok {
		t.Fatalf("unexpected token %#v", got)
	}
}
func TestDoesNotLexUppercasePrefix(t *testing.T) {
	got, ok := lexHexNumber([]rune("0Xf00d"))
	if ok {
		t.Fatalf("unexpected token %#v", got)
	}
}
func TestLexes0s(t *testing.T) {
	got, ok := lexPluralDigit([]rune("0s"))
	if !ok {
		t.Fatal("no token")
	}
	if got.Token.Kind != Word {
		t.Fatalf("token %#v", got)
	}
}
func TestLexes1ApostropheS(t *testing.T) {
	got, ok := lexPluralDigit([]rune("1's"))
	if !ok {
		t.Fatal("no token")
	}
	if got.Token.Kind != Word {
		t.Fatalf("token %#v", got)
	}
}
func TestLexes0sAnd1s(t *testing.T) {
	got, ok := lexPluralDigit([]rune("0s and 1s"))
	if !ok {
		t.Fatal("no token")
	}
	if got.Token.Kind != Word {
		t.Fatalf("token %#v", got)
	}
}
func TestLexes1sAnd0sApostrophes(t *testing.T) {
	got, ok := lexPluralDigit([]rune("1's and 0's"))
	if !ok {
		t.Fatal("no token")
	}
	if got.Token.Kind != Word {
		t.Fatalf("token %#v", got)
	}
}
func TestDoesntLex0sJoinedLetter(t *testing.T) {
	got, ok := lexPluralDigit([]rune("0ss"))
	if ok {
		t.Fatalf("unexpected token %#v", got)
	}
}
func TestDoesntLex1sApostropheJoinedNumber(t *testing.T) {
	got, ok := lexPluralDigit([]rune("1's1"))
	if ok {
		t.Fatalf("unexpected token %#v", got)
	}
}
func TestLexes20cDecade(t *testing.T) {
	got, ok := lexLongDecade([]rune("1980s"))
	if !ok {
		t.Fatal("no token")
	}
	if got.Token.Kind != Decade {
		t.Fatalf("token %#v", got)
	}
}
func TestLexes21cDecade(t *testing.T) {
	got, ok := lexLongDecade([]rune("2020s"))
	if !ok {
		t.Fatal("no token")
	}
	if got.Token.Kind != Decade {
		t.Fatalf("token %#v", got)
	}
}
func TestLexesAncientDecade(t *testing.T) {
	got, ok := lexLongDecade([]rune("1010s"))
	if !ok {
		t.Fatal("no token")
	}
	if got.Token.Kind != Decade {
		t.Fatalf("token %#v", got)
	}
}
func TestLexesWordBeforeDecade(t *testing.T) {
	got := LexEnglishToken([]rune("late 1980s"))
	if got.Token.Kind != Word {
		t.Fatalf("token %#v", got)
	}
}
func TestLexesWordAfterDecade(t *testing.T) {
	got := LexEnglishToken([]rune("1980s and"))
	if got.Token.Kind != Decade {
		t.Fatalf("token %#v", got)
	}
}
func TestDoesntLexFarFutureDecade(t *testing.T) {
	got, ok := lexLongDecade([]rune("3190s"))
	if ok {
		t.Fatalf("unexpected token %#v", got)
	}
}
func TestDoesntLexTooAncientDecade(t *testing.T) {
	got, ok := lexLongDecade([]rune("100s"))
	if ok {
		t.Fatalf("unexpected token %#v", got)
	}
}
func TestDoesntLex0PrefixedDecade(t *testing.T) {
	got, ok := lexLongDecade([]rune("0100s"))
	if ok {
		t.Fatalf("unexpected token %#v", got)
	}
}
func TestDoesntLexUppercaseDecade(t *testing.T) {
	got, ok := lexLongDecade([]rune("2000S"))
	if ok {
		t.Fatalf("unexpected token %#v", got)
	}
}
func TestDoesntLexOverlongDecade(t *testing.T) {
	got, ok := lexLongDecade([]rune("20000s"))
	if ok {
		t.Fatalf("unexpected token %#v", got)
	}
}
func TestDoesntLexApostropheLongDecade(t *testing.T) {
	got, ok := lexLongDecade([]rune("2020's"))
	if ok {
		t.Fatalf("unexpected token %#v", got)
	}
}
func TestDoesntLexBadApostropheShortDecade(t *testing.T) {
	got, ok := lexLongDecade([]rune("80's"))
	if ok {
		t.Fatalf("unexpected token %#v", got)
	}
}
func TestDoesntLexGoodApostropheShortDecade(t *testing.T) {
	got, ok := lexLongDecade([]rune("'90s"))
	if ok {
		t.Fatalf("unexpected token %#v", got)
	}
}
func TestAcceptsSentenceWithDecade(t *testing.T) {
	toks := PlainEnglish{}.Parse([]rune("To the early 1990s there were a lot of Movies where the bad guys were former Russian intelligence agents."))
	want := []TokenKindTag{Word, Space, Word, Space, Word, Space, Decade}
	for i, k := range want {
		if toks[i].Kind.Kind != k {
			t.Fatal(toks)
		}
	}
}
func TestRejectsSentenceWithNumber(t *testing.T) {
	toks := PlainEnglish{}.Parse([]rune("To the early 1990s there were a lot of Movies where the bad guys were former Russian intelligence agents."))
	for i, k := range []TokenKindTag{Word, Space, Word, Space, Word, Space} {
		if toks[i].Kind.Kind != k {
			t.Fatal(toks)
		}
	}
	if toks[6].Kind.IsNumber() {
		t.Fatal(toks)
	}
}
func TestIssue1010(t *testing.T) {
	got, ok := lexNumber([]rune("3."))
	if !ok || got.NextIndex != 1 {
		t.Fatal(got)
	}
}
func TestLexesFullNumber(t *testing.T) {
	got, ok := lexNumber([]rune("3.0"))
	if !ok || got.NextIndex != 3 {
		t.Fatal(got)
	}
}
func TestLexesNegative1(t *testing.T) {
	t.Skip("Negative numbers are not yet supported")
	got, ok := lexNumber([]rune("-1"))
	if !ok || !got.Token.IsNumber() {
		t.Fatal(got)
	}
}
func TestLexesPositive1(t *testing.T) {
	t.Skip("Positive numbers with a leading + are not supported")
	got, ok := lexNumber([]rune("+1"))
	if !ok || !got.Token.IsNumber() {
		t.Fatal(got)
	}
}
func TestRustDecimalNumberGrammar(t *testing.T) {
	for _, input := range []string{"0x1p2", "1_000"} {
		got, ok := lexNumber([]rune(input))
		if !ok || got.NextIndex != 1 {
			t.Fatal(input, got)
		}
	}
}
