package text

import (
	"math"
	"reflect"
	"testing"
)

func TestSpanOperations(t *testing.T) {
	s := NewSpanWithLen(2, 3)
	equal(t, s.Len(), 3)
	require(t, s.Contains(2))
	require(t, !s.Contains(5))
	equal(t, s.WithLen(1), Span{2, 3})
	equal(t, s.PushedBy(4), Span{6, 9})
	pulled, ok := s.PulledBy(2)
	require(t, ok)
	equal(t, pulled, Span{0, 3})
	_, ok = s.PulledBy(3)
	require(t, !ok)
	s.PushBy(2)
	s.PullBy(1)
	equal(t, s, Span{3, 6})
	s.SetLen(1)
	equal(t, s, Span{3, 4})
	content, ok := EmptySpan(100).TryGetContent([]rune("a"))
	require(t, ok)
	equal(t, len(content), 0)
	_, ok = NewSpan(1, 3).TryGetContent([]rune("a"))
	require(t, !ok)
	equal(t, string(NewSpan(1, 3).GetContent([]rune("abcd"))), "bc")
	var indices []int
	for i := range NewSpan(2, 5).Indices() {
		indices = append(indices, i)
	}
	require(t, reflect.DeepEqual(indices, []int{2, 3, 4}))
	count := 0
	for range NewSpan(0, 8).Indices() {
		count++
		break
	}
	equal(t, count, 1)
	mustPanic(t, func() { NewSpan(2, 1) })
	mustPanic(t, func() { NewSpan(0, 5).GetContent(nil) })
	require(t, reflect.DeepEqual(GetContent(Span{1, 3}, []int{1, 2, 3, 4}), []int{2, 3}))
}
func TestUnicodeCasing(t *testing.T) {
	equal(t, string(ToLower([]rune("İCAFÉ"))), "i\u0307café")
	equal(t, string(Upper.ApplyTo('ß')), "SS")
	equal(t, string(Upper.ApplyTo('ﬃ')), "FFI")
	for _, tc := range []struct{ template, target, want string }{{"", "HELLO", "hello"}, {"A", "hello world", "HELLO WORLD"}, {"Ab", "hello", "Hello"}, {"aB", "hey there", "hEY THERE"}, {"A-b", "a-b-c", "A-b-c"}, {"AB", "ßﬃ", "SSFFI"}} {
		equal(t, string(CopyCasing([]rune(tc.template), []rune(tc.target))), tc.want)
	}
	require(t, reflect.DeepEqual(GetCasing([]rune("A1b!")), []Case{Upper, Lower}))
	unfiltered := GetCasingUnfiltered([]rune("A1b!"))
	equal(t, len(unfiltered), 4)
	equal(t, *unfiltered[0], Upper)
	require(t, unfiltered[1] == nil)
	equal(t, *unfiltered[2], Lower)
	require(t, unfiltered[3] == nil)
	casing, ok := CaseFromChar('Ⅰ')
	require(t, ok)
	equal(t, casing, Upper)
	_, ok = CaseFromChar('!')
	require(t, !ok)
	original := []rune("abc")
	lower := ToLower(original)
	require(t, &lower[0] == &original[0])
}
func TestCharacterHelpers(t *testing.T) {
	for _, c := range []rune("abcXYZÀéǆ") {
		require(t, IsEnglishLingual(c))
	}
	for _, c := range []rune("世🙂中日한あ1!? ") {
		require(t, !IsEnglishLingual(c))
	}
	for _, c := range []rune("☀😀\ufe0f\ufffd🦀") {
		require(t, IsEmoji(c))
	}
	require(t, !IsEmoji('A'))
	require(t, !IsEmoji('🚀'))
	for _, c := range []rune("AeiOu") {
		require(t, IsVowel(c))
	}
	require(t, !IsVowel('é'))
	require(t, ContainsVowel([]rune("rhythmA")))
	require(t, !ContainsVowel([]rune("rhythm")))
	equal(t, string(Normalized([]rune("‘’ʼ＇“”＂–—−－"))), "''''\"\"\"----")
	chars := []rune("plain")
	require(t, &Normalized(chars)[0] == &chars[0])
	equal(t, ToString(chars), "plain")
	require(t, EqAnyIgnoreASCIICaseStr([]rune("FOO"), []string{"bar", "foo"}))
	require(t, EqAnyIgnoreASCIICaseChars([]rune("FOO"), [][]rune{[]rune("bar"), []rune("foo")}))
	require(t, StartsWithAnyIgnoreASCIICaseStr([]rune("HELLO"), []string{"no", "he"}))
	require(t, EndsWithAnyIgnoreASCIICaseChars([]rune("HELLO"), [][]rune{[]rune("lo")}))
	require(t, !EndsWithIgnoreASCIICaseStr([]rune("Hello"), "LO"))
	require(t, !EndsWithIgnoreASCIICaseChars([]rune("Hello"), []rune("LO")))
	rest, ok := StripPrefixIgnoreASCIICaseChars([]rune("Hello"), []rune("HE"))
	require(t, ok)
	equal(t, string(rest), "llo")
	_, ok = StripPrefixIgnoreASCIICaseChars([]rune("a"), []rune("abc"))
	require(t, !ok)
	require(t, !EqStrLenient([]rune("É"), "é"))
}
func TestOrdinalSuffixes(t *testing.T) {
	for _, tc := range []struct {
		n float64
		s OrdinalSuffix
	}{{0, Th}, {1, St}, {2, Nd}, {3, Rd}, {4, Th}, {11, Th}, {12, Th}, {13, Th}, {21, St}, {112, Th}} {
		got, ok := CorrectSuffixFor(tc.n)
		require(t, ok)
		equal(t, got, tc.s)
		parsed, ok := OrdinalSuffixFromChars(tc.s.ToChars())
		require(t, ok)
		equal(t, parsed, tc.s)
	}
	for _, n := range []float64{-1, 1.5, math.Inf(1)} {
		_, ok := CorrectSuffixFor(n)
		require(t, !ok)
	}
	for _, s := range []string{"th", "TH", "st", "sT", "Nd", "RD"} {
		_, ok := OrdinalSuffixFromChars([]rune(s))
		require(t, ok)
	}
	equal(t, (Number{Value: -1, Radix: 16}).String(), "0x0")
	equal(t, (Number{Value: math.Inf(1), Radix: 16}).String(), "0xFFFFFFFFFFFFFFFF")
}
func TestReusableEditDistance(t *testing.T) {
	var previous, current []uint8
	equal(t, EditDistanceMinAlloc([]rune("kitten"), []rune("sitting"), &previous, &current), uint8(3))
	equal(t, EditDistanceMinAlloc([]rune("é"), []rune("e"), &previous, &current), uint8(1))
	equal(t, EditDistance(nil, []rune("abc")), uint8(3))
	equal(t, EditDistance([]rune("abc"), nil), uint8(3))
}

// These primitive values and rune ranges were observed with Harper 3.2.0's parse command.
func TestOraclePrimitiveFixture(t *testing.T) {
	source := []rune("Hello world! À café 世 🙂 15th $20 — a’b “test”")
	equal(t, (Span{24, 28}).GetContentString(source), "15th")
	equal(t, (Number{Value: 15, Suffix: func() *OrdinalSuffix { v := Th; return &v }(), Radix: 10}).String(), "15th")
	p, ok := PunctuationFromChar(source[29])
	require(t, ok)
	equal(t, p.Kind, CurrencyPunctuation)
	equal(t, p.Currency, Dollar)
	p, ok = PunctuationFromChar(source[33])
	require(t, ok)
	equal(t, p.Kind, EmDash)
	require(t, !IsEnglishLingual(source[20]))
	require(t, !IsEnglishLingual(source[22]))
	require(t, IsEnglishLingual(source[13]))
	equal(t, (Span{0, 5}).GetContentString(source), "Hello")
	equal(t, (Span{6, 11}).GetContentString(source), "world")
}
