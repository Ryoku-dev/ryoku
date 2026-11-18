package dict

import "testing"

func TestOrthographyTestLowercaseFlags(t *testing.T) {
	flags := OrthFlagsFromLetters([]rune("hello"))
	if !(flags.Contains(LOWERCASE)) {
		t.Fatal("orthography assertion failed")
	}
	if !(!flags.Contains(TITLECASE)) {
		t.Fatal("orthography assertion failed")
	}
	if !(!flags.Contains(ALLCAPS)) {
		t.Fatal("orthography assertion failed")
	}
	if !(!flags.Contains(LOWER_CAMEL)) {
		t.Fatal("orthography assertion failed")
	}
	if !(!flags.Contains(UPPER_CAMEL)) {
		t.Fatal("orthography assertion failed")
	}

	flags = OrthFlagsFromLetters([]rune("hello123"))
	if !(flags.Contains(LOWERCASE)) {
		t.Fatal("orthography assertion failed")
	}
}
func TestOrthographyTestTitlecaseFlags(t *testing.T) {
	flags := OrthFlagsFromLetters([]rune("Hello"))
	if !(!flags.Contains(LOWERCASE)) {
		t.Fatal("orthography assertion failed")
	}
	if !(flags.Contains(TITLECASE)) {
		t.Fatal("orthography assertion failed")
	}
	if !(!flags.Contains(ALLCAPS)) {
		t.Fatal("orthography assertion failed")
	}
	if !(!flags.Contains(LOWER_CAMEL)) {
		t.Fatal("orthography assertion failed")
	}
	if !(!flags.Contains(UPPER_CAMEL)) {
		t.Fatal("orthography assertion failed")
	}

	if !(OrthFlagsFromLetters([]rune("World")).Contains(TITLECASE)) {
		t.Fatal("orthography assertion failed")
	}
	if !(OrthFlagsFromLetters([]rune("Something")).Contains(TITLECASE)) {
		t.Fatal("orthography assertion failed")
	}
	if !(!OrthFlagsFromLetters([]rune("McDonald")).Contains(TITLECASE)) {
		t.Fatal("orthography assertion failed")
	}
	if !(!OrthFlagsFromLetters([]rune("O'Reilly")).Contains(TITLECASE)) {
		t.Fatal("orthography assertion failed")
	}
	if !(!OrthFlagsFromLetters([]rune("A")).Contains(TITLECASE)) {
		t.Fatal("orthography assertion failed")
	}
}
func TestOrthographyTestAllcapsFlags(t *testing.T) {
	flags := OrthFlagsFromLetters([]rune("HELLO"))
	if !(!flags.Contains(LOWERCASE)) {
		t.Fatal("orthography assertion failed")
	}
	if !(!flags.Contains(TITLECASE)) {
		t.Fatal("orthography assertion failed")
	}
	if !(flags.Contains(ALLCAPS)) {
		t.Fatal("orthography assertion failed")
	}
	if !(!flags.Contains(LOWER_CAMEL)) {
		t.Fatal("orthography assertion failed")
	}
	if !(!flags.Contains(UPPER_CAMEL)) {
		t.Fatal("orthography assertion failed")
	}

	if !(OrthFlagsFromLetters([]rune("NASA")).Contains(ALLCAPS)) {
		t.Fatal("orthography assertion failed")
	}
	if !(OrthFlagsFromLetters([]rune("I")).Contains(ALLCAPS)) {
		t.Fatal("orthography assertion failed")
	}
}
func TestOrthographyTestLowerCamelFlags(t *testing.T) {
	flags := OrthFlagsFromLetters([]rune("helloWorld"))
	if !(!flags.Contains(LOWERCASE)) {
		t.Fatal("orthography assertion failed")
	}
	if !(!flags.Contains(TITLECASE)) {
		t.Fatal("orthography assertion failed")
	}
	if !(!flags.Contains(ALLCAPS)) {
		t.Fatal("orthography assertion failed")
	}
	if !(flags.Contains(LOWER_CAMEL)) {
		t.Fatal("orthography assertion failed")
	}
	if !(!flags.Contains(UPPER_CAMEL)) {
		t.Fatal("orthography assertion failed")
	}

	if !(OrthFlagsFromLetters([]rune("getHTTPResponse")).Contains(LOWER_CAMEL)) {
		t.Fatal("orthography assertion failed")
	}
	if !(OrthFlagsFromLetters([]rune("eBay")).Contains(LOWER_CAMEL)) {
		t.Fatal("orthography assertion failed")
	}
	if !(!OrthFlagsFromLetters([]rune("hello")).Contains(LOWER_CAMEL)) {
		t.Fatal("orthography assertion failed")
	}
	if !(!OrthFlagsFromLetters([]rune("HelloWorld")).Contains(LOWER_CAMEL)) {
		t.Fatal("orthography assertion failed")
	}
}
func TestOrthographyTestUpperCamelFlags(t *testing.T) {
	flags := OrthFlagsFromLetters([]rune("HelloWorld"))
	if !(!flags.Contains(LOWERCASE)) {
		t.Fatal("orthography assertion failed")
	}
	if !(!flags.Contains(TITLECASE)) {
		t.Fatal("orthography assertion failed")
	}
	if !(!flags.Contains(ALLCAPS)) {
		t.Fatal("orthography assertion failed")
	}
	if !(!flags.Contains(LOWER_CAMEL)) {
		t.Fatal("orthography assertion failed")
	}
	if !(flags.Contains(UPPER_CAMEL)) {
		t.Fatal("orthography assertion failed")
	}

	if !(OrthFlagsFromLetters([]rune("HttpRequest")).Contains(UPPER_CAMEL)) {
		t.Fatal("orthography assertion failed")
	}
	if !(OrthFlagsFromLetters([]rune("McDonald")).Contains(UPPER_CAMEL)) {
		t.Fatal("orthography assertion failed")
	}
	if !(OrthFlagsFromLetters([]rune("O'Reilly")).Contains(UPPER_CAMEL)) {
		t.Fatal("orthography assertion failed")
	}
	if !(OrthFlagsFromLetters([]rune("XMLHttpRequest")).Contains(UPPER_CAMEL)) {
		t.Fatal("orthography assertion failed")
	}
	if !(!OrthFlagsFromLetters([]rune("Hello")).Contains(UPPER_CAMEL)) {
		t.Fatal("orthography assertion failed")
	}
	if !(!OrthFlagsFromLetters([]rune("NASA")).Contains(UPPER_CAMEL)) {
		t.Fatal("orthography assertion failed")
	}
	if !(!OrthFlagsFromLetters([]rune("Hi")).Contains(UPPER_CAMEL)) {
		t.Fatal("orthography assertion failed")
	}
}
func TestOrthographyTestRomanNumeralFlags(t *testing.T) {
	if !(OrthFlagsFromLetters([]rune("MCMXCIV")).Contains(ROMAN_NUMERALS)) {
		t.Fatal("orthography assertion failed")
	}
	if !(OrthFlagsFromLetters([]rune("mdccclxxi")).Contains(ROMAN_NUMERALS)) {
		t.Fatal("orthography assertion failed")
	}
	if !(OrthFlagsFromLetters([]rune("MMXXI")).Contains(ROMAN_NUMERALS)) {
		t.Fatal("orthography assertion failed")
	}
	if !(OrthFlagsFromLetters([]rune("mcmxciv")).Contains(ROMAN_NUMERALS)) {
		t.Fatal("orthography assertion failed")
	}
	if !(OrthFlagsFromLetters([]rune("MCMXCIV")).Contains(ROMAN_NUMERALS)) {
		t.Fatal("orthography assertion failed")
	}
	if !(OrthFlagsFromLetters([]rune("MMI")).Contains(ROMAN_NUMERALS)) {
		t.Fatal("orthography assertion failed")
	}
	if !(OrthFlagsFromLetters([]rune("MMXXV")).Contains(ROMAN_NUMERALS)) {
		t.Fatal("orthography assertion failed")
	}
}
func TestOrthographyTestSingleRomanNumeralFlags(t *testing.T) {
	if !(OrthFlagsFromLetters([]rune("i")).Contains(ROMAN_NUMERALS)) {
		t.Fatal("orthography assertion failed")
	}
}
func TestOrthographyEmptyStringIsNotRomanNumeral(t *testing.T) {
	if !(!OrthFlagsFromLetters([]rune("")).Contains(ROMAN_NUMERALS)) {
		t.Fatal("orthography assertion failed")
	}
}
func TestOrthographyDontAllowMixedCaseRomanNumerals(t *testing.T) {
	if !(!OrthFlagsFromLetters([]rune("MCMlxxxVIII")).Contains(ROMAN_NUMERALS)) {
		t.Fatal("orthography assertion failed")
	}
}
func TestOrthographyDontAllowLooksLikeButIsntRomanNumeral(t *testing.T) {
	if !(!OrthFlagsFromLetters([]rune("mdxlivx")).Contains(ROMAN_NUMERALS)) {
		t.Fatal("orthography assertion failed")
	}
	if !(!OrthFlagsFromLetters([]rune("XIXIVV")).Contains(ROMAN_NUMERALS)) {
		t.Fatal("orthography assertion failed")
	}
}
func TestOrthographyAustraliaLexemeIsTitlecaseEvenWhenWordIsLowercase(t *testing.T) {
	if !(testMD(t, "australia").OrthInfo.Contains(TITLECASE)) {
		t.Fatal("orthography assertion failed")
	}
}
func TestOrthographyAustraliaLexemeIsTitlecaseEvenWhenWordIsAllCaps(t *testing.T) {
	if !(testMD(t, "AUSTRALIA").OrthInfo.Contains(TITLECASE)) {
		t.Fatal("orthography assertion failed")
	}
}
func TestOrthographyAustraliaLexemeIsTitlecaseEvenWhenWordIsMixedCase(t *testing.T) {
	if !(testMD(t, "AuStrAlIA").OrthInfo.Contains(TITLECASE)) {
		t.Fatal("orthography assertion failed")
	}
}
func TestOrthographyDbAndKwSymbolsAreLowerCamelCase(t *testing.T) {

	if !(testMD(t, "db").OrthInfo.Contains(LOWER_CAMEL)) {
		t.Fatal("orthography assertion failed")
	}
}
func TestOrthographyAmIsLowercaseAndTitlecaseAndAllCaps(t *testing.T) {

	metadata := testMD(t, "am")
	if !(metadata.OrthInfo.Contains(LOWERCASE)) {
		t.Fatal("orthography assertion failed")
	}
	if !(metadata.OrthInfo.Contains(TITLECASE)) {
		t.Fatal("orthography assertion failed")
	}
	if !(metadata.OrthInfo.Contains(ALLCAPS)) {
		t.Fatal("orthography assertion failed")
	}
}
func TestOrthographyReadingIsBothLowercaseAndTitlecase(t *testing.T) {

	metadata := testMD(t, "reading")
	if !(metadata.OrthInfo.Contains(LOWERCASE)) {
		t.Fatal("orthography assertion failed")
	}
	if !(metadata.OrthInfo.Contains(TITLECASE)) {
		t.Fatal("orthography assertion failed")
	}
}
func TestOrthographyEbayAndEsimAreLowerCamel(t *testing.T) {

	md1 := testMD(t, "ebay")
	if !(md1.OrthInfo.Contains(LOWER_CAMEL)) {
		t.Fatal("orthography assertion failed")
	}
	md2 := testMD(t, "esim")
	if !(md2.OrthInfo.Contains(LOWER_CAMEL)) {
		t.Fatal("orthography assertion failed")
	}
}
