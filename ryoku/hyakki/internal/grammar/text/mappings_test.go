package text

import "testing"

func TestEveryPunctuationMapping(t *testing.T) {
	cases := []struct {
		c rune
		k PunctuationKind
	}{{180, Acute}, {38, Ampersand}, {8217, Apostrophe}, {39, Apostrophe}, {64, At}, {92, Backslash}, {33, Bang}, {94, Caret}, {58, Colon}, {44, Comma}, {12289, Comma}, {65292, Comma}, {176, Degree}, {8243, DoublePrime}, {8211, EnDash}, {8212, EmDash}, {8230, Ellipsis}, {61, Equal}, {47, ForwardSlash}, {62, GreaterThan}, {35, Hash}, {45, Hyphen}, {60, LessThan}, {8216, OpenSingle}, {37, Percent}, {124, Pipe}, {43, Plus}, {63, Question}, {46, Period}, {59, Semicolon}, {8242, SinglePrime}, {42, Star}, {126, Tilde}, {95, Underscore}, {91, OpenSquare}, {93, CloseSquare}, {123, OpenCurly}, {125, CloseCurly}, {40, OpenRound}, {41, CloseRound}, {96, Backtick}}
	for _, tc := range cases {
		got, ok := PunctuationFromChar(tc.c)
		require(t, ok)
		equal(t, got.Kind, tc.k)
		require(t, IsPunctuation(tc.c))
	}
	for _, c := range []rune{'a', '中', '"', '“', '”'} {
		_, ok := PunctuationFromChar(c)
		require(t, !ok)
	}
}
func TestEveryCurrencyMapping(t *testing.T) {
	cases := []struct {
		c        rune
		currency Currency
		amount   string
	}{{36, Dollar, "$20"}, {162, Cent, "20¢"}, {8364, Euro, "€20"}, {8381, Ruble, "20 ₽"}, {8378, Lira, "20 ₺"}, {163, Pound, "£20"}, {165, Yen, "¥20"}, {3647, Baht, "20 ฿"}, {8361, Won, "₩ 20"}, {8365, Kip, "₭20"}, {8377, Rupee, "₹20"}}
	for _, tc := range cases {
		got, ok := CurrencyFromChar(tc.c)
		require(t, ok)
		equal(t, got, tc.currency)
		equal(t, got.ToChar(), tc.c)
		equal(t, got.FormatAmount(Number{Value: 20, Radix: 10}), tc.amount)
		punct, ok := PunctuationFromChar(tc.c)
		require(t, ok)
		equal(t, punct.Kind, CurrencyPunctuation)
		equal(t, punct.Currency, tc.currency)
	}
	_, ok := CurrencyFromChar('a')
	require(t, !ok)
}
