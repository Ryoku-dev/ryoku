package expr

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/text"
	"testing"
)

func TestWordFruit(t *testing.T) {
	d := doc.NewPlainEnglishCurated("I ate a banana and an apple today.")
	assertSpans(t, FindAllMatchesInDoc(NewWord("banana"), d), []text.Span{{Start: 6, End: 7}})
	assertSpans(t, FindAllMatchesInDoc(NewWordExact("banana"), d), []text.Span{{Start: 6, End: 7}})
}
func TestWordFruitWhackCapitalization(t *testing.T) {
	d := doc.NewPlainEnglishCurated("I Ate A bAnaNa And aN apPlE today.")
	assertSpans(t, FindAllMatchesInDoc(NewWord("banana"), d), []text.Span{{Start: 6, End: 7}})
	assertSpans(t, FindAllMatchesInDoc(NewWordExact("banana"), d), nil)
}
func TestStandardCaseBasicMatches(t *testing.T) {
	assertStrings(t, NewWordStandardCase("banana"), "I ate a banana and BANANA and Banana today.", []string{"banana", "BANANA", "Banana"})
}
func TestStandardCaseRejectsMixedCase(t *testing.T) {
	for _, word := range []string{"iphone", "ipad", "youtube"} {
		assertNoMatches(t, NewWordStandardCase(word), "I saw iPhone and iPad and YouTube today.")
	}
}
func TestStandardCaseRejectsPascalAndCamelCase(t *testing.T) {
	for _, word := range []string{"bananatree", "banana"} {
		assertNoMatches(t, NewWordStandardCase(word), "I saw BananaTree and bananaTree and BaNaNa today.")
	}
}
func TestStandardCaseSingleLetters(t *testing.T) {
	input := "A B C a b c I i"
	assertStrings(t, NewWordStandardCase("a"), input, []string{"A", "a"})
	assertStrings(t, NewWordStandardCase("b"), input, []string{"B", "b"})
	assertStrings(t, NewWordStandardCase("i"), input, []string{"I", "i"})
}
func TestStandardCaseVsExactCase(t *testing.T) {
	input := "I ate banana and BANANA and Banana."
	assertStrings(t, NewWordExact("banana"), input, []string{"banana"})
	assertStrings(t, NewWordStandardCase("banana"), input, []string{"banana", "BANANA", "Banana"})
}
func TestStandardCaseEdgeCases(t *testing.T) {
	input := "A a B b I i."
	assertStrings(t, NewWordStandardCase("a"), input, []string{"A", "a"})
	assertStrings(t, NewWordStandardCase("i"), input, []string{"I", "i"})
}
func TestStandardCaseComplexExamples(t *testing.T) {
	input := "The iPhone is made by Apple but the apple is fruit."
	assertStrings(t, NewWordStandardCase("apple"), input, []string{"Apple", "apple"})
	assertNoMatches(t, NewWordStandardCase("iphone"), input)
}
func TestMatchAllStandard3LetterSets(t *testing.T) {
	assertStrings(t, NewWordStandardCase("abc"), "abc abC aBc aBC Abc AbC ABc ABC", []string{"abc", "Abc", "ABC"})
}
func TestWordSetFruit(t *testing.T) {
	set := NewWordSet([]string{"banana", "apple", "orange"})
	d := doc.NewPlainEnglishCurated("I ate a banana and an apple today.")
	assertSpans(t, FindAllMatchesInDoc(set, d), []text.Span{{Start: 6, End: 7}, {Start: 12, End: 13}})
}
func TestWordSetFruitWhackCapitalization(t *testing.T) {
	set := NewWordSet([]string{"banana", "apple", "orange"})
	d := doc.NewPlainEnglishCurated("I Ate A bAnaNa And aN apPlE today.")
	assertSpans(t, FindAllMatchesInDoc(set, d), []text.Span{{Start: 6, End: 7}, {Start: 12, End: 13}})
}
func TestSupportsTypographicApostrophes(t *testing.T) {
	set := NewWordSet([]string{"They're"})
	d := doc.NewPlainEnglishCurated("They’re")
	assertSpans(t, FindAllMatchesInDoc(set, d), []text.Span{{Start: 0, End: 1}})
}
func TestNumberImplies(t *testing.T) {
	d := doc.NewPlainEnglishCurated("There are 60 minutes in an hour.")
	assertSpans(t, FindAllMatchesInDoc(NewImpliesQuantity(), d), []text.Span{{Start: 4, End: 5}, {Start: 10, End: 11}})
}
func TestSimpleApple(t *testing.T) {
	assertStrings(t, NewNominalPhrase(), "A red apple", []string{"A red apple"})
}
func TestComplexApple(t *testing.T) {
	assertStrings(t, NewNominalPhrase(), "A red apple with a long stem", []string{"A red apple", "a long stem"})
}
func TestListFruit(t *testing.T) {
	assertStrings(t, NewNominalPhrase(), "An apple, a banana and a pear", []string{"An apple", "a banana", "a pear"})
}
func assertNominalMatches(t *testing.T, input string) {
	t.Helper()
	d := doc.NewPlainEnglishCurated(input)
	if _, ok := NewNominalPhrase().Matches(d.GetTokens(), d.GetSource()); !ok {
		t.Fatalf("%q did not match", input)
	}
}
func TestSimplestBanana(t *testing.T) { assertNominalMatches(t, "a banana") }
func TestFood(t *testing.T) {
	assertStrings(t, NewNominalPhrase(), "My favorite foods are pizza, sushi, tacos and burgers.", []string{"My favorite foods", "pizza", "sushi", "tacos", "burgers"})
}
func TestSimplestWay(t *testing.T)          { assertNominalMatches(t, "a way") }
func TestPresentParticipleWay(t *testing.T) { assertNominalMatches(t, "a winning way") }
func TestPerfectParticipleWay(t *testing.T) { assertNominalMatches(t, "a failed way") }
