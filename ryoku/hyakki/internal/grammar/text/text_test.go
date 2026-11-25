package text

import (
	"reflect"
	"testing"
)

func require(t *testing.T, ok bool) {
	t.Helper()
	if !ok {
		t.Fatal("assertion failed")
	}
}
func equal[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}
func mustPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	fn()
}

func TestEqIgnoreAsciiCaseCharsMatchesLowercase(t *testing.T) {
	require(t, EqCh([]rune("Hello"), []rune("hello")))
}
func TestEqIgnoreAsciiCaseCharsDoesNotMatchDifferentWord(t *testing.T) {
	require(t, !EqCh([]rune("Hello"), []rune("world")))
}
func TestEqIgnoreAsciiCaseStrMatchesLowercase(t *testing.T) {
	require(t, EqStr([]rune("Hello"), "hello"))
}
func TestEqIgnoreAsciiCaseStrDoesNotMatchDifferentWord(t *testing.T) {
	require(t, !EqStr([]rune("Hello"), "world"))
}
func TestEndsWithIgnoreAsciiCaseCharsMatchesSuffix(t *testing.T) {
	require(t, EndsWithIgnoreASCIICaseChars([]rune("Hello"), []rune("lo")))
}
func TestEndsWithIgnoreAsciiCaseCharsDoesNotMatchDifferentSuffix(t *testing.T) {
	require(t, !EndsWithIgnoreASCIICaseChars([]rune("Hello"), []rune("world")))
}
func TestEndsWithIgnoreAsciiCaseStrMatchesSuffix(t *testing.T) {
	require(t, EndsWithIgnoreASCIICaseStr([]rune("Hello"), "lo"))
}
func TestEndsWithIgnoreAsciiCaseStrDoesNotMatchDifferentSuffix(t *testing.T) {
	require(t, !EndsWithIgnoreASCIICaseStr([]rune("Hello"), "world"))
}
func TestDiffersOnlyByLength1(t *testing.T) { require(t, !EqStr([]rune("bb"), "b")) }
func TestDiffersOnlyByLength2(t *testing.T) { require(t, !EqStr([]rune("c"), "cc")) }
func TestRightSideMustBeAllLowercaseStr(t *testing.T) {
	mustPanic(t, func() { require(t, EqStr([]rune("c"), "C")) })
}
func TestRightSideMustBeAllLowercaseCh(t *testing.T) {
	mustPanic(t, func() { require(t, EqCh([]rune("c"), []rune("C"))) })
}
func TestLenientRightSideCanBeAllLowercaseStr(t *testing.T) {
	require(t, EqStrLenient([]rune("Hello"), "hello"))
}
func TestLenientRightSideCanBeMixedCaseStr(t *testing.T) {
	require(t, EqStrLenient([]rune("Hello"), "HeLlO"))
}
func TestLenientRightSideCanBeAllLowercaseCh(t *testing.T) {
	require(t, EqChLenient([]rune("Hello"), []rune("hello")))
}
func TestLenientRightSideCanBeMixedCaseCh(t *testing.T) {
	require(t, EqChLenient([]rune("Hello"), []rune("HELLO")))
}
func TestCjkIsNotEnglishLingual(t *testing.T) { require(t, !IsEnglishLingual('世')) }

func TestHexFifteen(t *testing.T)     { equal(t, (Number{Value: 15, Radix: 16}).String(), "0xF") }
func TestDecimalFifteen(t *testing.T) { equal(t, (Number{Value: 15, Radix: 10}).String(), "15") }
func TestDecimalFifteenSuffix(t *testing.T) {
	suffix := Th
	equal(t, (Number{Value: 15, Suffix: &suffix, Radix: 10}).String(), "15th")
}
func TestDecimalFifteenAndAHalf(t *testing.T) {
	equal(t, (Number{Value: 15.5, Radix: 10, Precision: 2}).String(), "15.50")
}
func TestIssue1051(t *testing.T) { _, ok := OrdinalSuffixFromChars([]rune("story")); require(t, !ok) }

func TestSimpleEditDistance1(t *testing.T) {
	equal(t, EditDistance([]rune("kitten"), []rune("sitting")), uint8(3))
}
func TestSimpleEditDistance2(t *testing.T) {
	equal(t, EditDistance([]rune("saturday"), []rune("sunday")), uint8(3))
}
func TestOneEditDistance(t *testing.T) {
	for _, target := range []string{"hellos", "hell", "hell", "hvllo", "Hello"} {
		equal(t, EditDistance([]rune("hello"), []rune(target)), uint8(1))
	}
}
func TestZeroEditDistance(t *testing.T) {
	equal(t, EditDistance([]rune("hello"), []rune("hello")), uint8(0))
}
func TestRemovesRequestedIndices(t *testing.T) {
	data := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	data = RemoveIndices(data, []int{1, 4, 6})
	require(t, reflect.DeepEqual(data, []int{0, 2, 3, 5, 7, 8, 9}))
}
func TestMapsAsciiOffsets(t *testing.T) {
	require(t, reflect.DeepEqual(BuildByteToCharMap("abc"), []int{0, 1, 2, 3}))
}
func TestMapsUnicodeOffsets(t *testing.T) {
	require(t, reflect.DeepEqual(BuildByteToCharMap("A🙂B"), []int{0, 1, 1, 1, 1, 2, 3}))
}
func TestOverlaps(t *testing.T) {
	require(t, NewSpan(0, 5).OverlapsWith(NewSpan(3, 6)))
	require(t, NewSpan(0, 5).OverlapsWith(NewSpan(2, 3)))
	require(t, NewSpan(0, 5).OverlapsWith(NewSpan(4, 5)))
	require(t, NewSpan(0, 5).OverlapsWith(NewSpan(4, 4)))
	require(t, !NewSpan(0, 3).OverlapsWith(NewSpan(3, 5)))
}
func TestExpandsProperly(t *testing.T) {
	span := NewSpan(2, 2)
	span.ExpandToInclude(1)
	equal(t, span, NewSpan(1, 2))
	span.ExpandToInclude(2)
	equal(t, span, NewSpan(1, 3))
}
func TestToCharSpanConvertsCorrectly(t *testing.T) {
	source := []rune("Hello world!")
	tokenSpans := []Span{{0, 5}, {5, 6}, {6, 11}, {11, 12}}
	require(t, Zero.ToCharSpan(tokenSpans).IsEmpty())
	converted := NewSpan(0, 3).ToCharSpan(tokenSpans)
	equal(t, converted.GetContentString(source), "Hello world")
}
