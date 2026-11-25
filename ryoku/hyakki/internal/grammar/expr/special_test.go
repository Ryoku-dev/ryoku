package expr

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"testing"
)

func assertMatchCount(t *testing.T, e Expr, input string, want int) {
	t.Helper()
	m := IterMatchesInDoc(e, doc.NewPlainEnglishCurated(input))
	if len(m) != want {
		t.Fatalf("%q got %d matches %v, want %d", input, len(m), m, want)
	}
}
func TestOkIAm(t *testing.T) {
	assertMatchCount(t, NewPronounBe(), "I am an auditor of software in M&A.", 1)
}
func TestOkIM(t *testing.T) {
	assertMatchCount(t, NewPronounBe(), "I'm using Beads in every new project and adding it to every old project I visit with an agent.", 1)
}
func TestOkWeAre(t *testing.T) {
	assertMatchCount(t, NewPronounBe(), "Error: we are enable to complete your request at this time.", 1)
}
func TestOkWeRe(t *testing.T) {
	assertMatchCount(t, NewPronounBe(), "We're currently experiencing high demand, which may cause temporary errors.", 1)
}
func TestOkYouAre(t *testing.T) {
	assertMatchCount(t, NewPronounBe(), "You are Dolphin, an uncensored and unbiased AI assistant.", 1)
}
func TestOkYouRe(t *testing.T) {
	assertMatchCount(t, NewPronounBe(), "You're trying to use a SD1 LoRA model with a SDXL Stable Diffusion model.", 1)
}
func TestOkHeIs(t *testing.T) {
	assertMatchCount(t, NewPronounBe(), "He is just a modulary guy for checking service health.", 1)
}
func TestOkHeSAndItIs(t *testing.T) {
	assertMatchCount(t, NewPronounBe(), "He's Dead, Jim is a link checking program, specifically it is a command-line tool for finding and reporting dead links", 2)
}
func TestOkSheIs(t *testing.T) {
	assertMatchCount(t, NewPronounBe(), "SHE is designed by following the SHE functional specification.", 1)
}
func TestOkSheS(t *testing.T) {
	assertMatchCount(t, NewPronounBe(), "She's Coding is an open-source website project currently under development in cooperation with the documentary film CODE: Debugging the Gender Gap.", 1)
}
func TestOkItIs(t *testing.T) {
	assertMatchCount(t, NewPronounBe(), "It is same as ms-dos command where.exe.", 1)
}
func TestOkItSAndYouAre(t *testing.T) {
	assertMatchCount(t, NewPronounBe(), "It's not working · Microcontrollers have only a limited amount of RAM: Verify that you are not running out of available RAM!", 2)
}
func TestOkTheyAre(t *testing.T) {
	assertMatchCount(t, NewPronounBe(), "Multidist binaries ignore the way they are called and check only the running binary name, making them hard to invoke from a single one.", 1)
}
func TestOkTheyRe(t *testing.T) {
	assertMatchCount(t, NewPronounBe(), "Cannot load dataset, they're greyed out.", 1)
}
func TestVariousApostrophes(t *testing.T) {
	assertMatchCount(t, NewPronounBe(), "it's/it’s", 2)
	assertMatchCount(t, NewPronounBe(), "it;s/it´s", 0)
}
func TestDetect10Days(t *testing.T) {
	assertStrings(t, NewDurationExpr(), "Is 10 days a long time?", []string{"10 days"})
}
func TestDetectTenDays(t *testing.T) {
	assertStrings(t, NewDurationExpr(), "I think ten days is a long time.", []string{"ten days"})
}
func TestDetectAFewMonths(t *testing.T) {
	assertStrings(t, NewDurationExpr(), "I am struggling since a few months with the rebuild of an old FORTRAN program.", []string{"a few months"})
}
func TestMatchesSingleDigit(t *testing.T) {
	assertMatchCount(t, NewSpelledNumberExpr(), "one two three", 3)
}
func TestMatchesTeens(t *testing.T) {
	assertMatchCount(t, NewSpelledNumberExpr(), "ten eleven twelve", 3)
}
func TestMatchesTens(t *testing.T) {
	assertMatchCount(t, NewSpelledNumberExpr(), "twenty thirty forty", 3)
}
func TestMatchesCompoundNumbers(t *testing.T) {
	assertMatchCount(t, NewSpelledNumberExpr(), "twenty-one thirty-two", 2)
}
func TestDeepThought(t *testing.T) {
	assertStrings(t, NewSpelledNumberExpr(), "the answer to the ultimate question of life, the universe, and everything is forty-two", []string{"forty-two"})
}
func TestJacksons(t *testing.T) {
	assertStrings(t, NewSpelledNumberExpr(), "A, B, C It's easy as one, two, three. Or simple as Do-Re-Mi", []string{"one", "two", "three"})
}
func TestOrwell(t *testing.T) {
	assertStrings(t, NewSpelledNumberExpr(), "Nineteen Eighty-Four", []string{"Nineteen", "Eighty-Four"})
}
func TestGetSmart(t *testing.T) {
	assertStrings(t, NewSpelledNumberExpr(), "Maxwell Smart was Agent Eighty-Six, but who was Agent Ninety-Nine?", []string{"Eighty-Six", "Ninety-Nine"})
}
func TestHyphensOrSpaces(t *testing.T) {
	assertStrings(t, NewSpelledNumberExpr(), "twenty-one, thirty two, forty-three, fifty four, sixty-five, seventy six, eighty-seven, ninety eight", []string{"twenty-one", "thirty two", "forty-three", "fifty four", "sixty-five", "seventy six", "eighty-seven", "ninety eight"})
}
func TestWaitingSince(t *testing.T) {
	assertStrings(t, NewSpelledNumberExpr(), "I have been waiting since two hours.", []string{"two"})
}
func mergePredicate(closed, open *dict.DictWordMetadata) bool {
	return open == nil && closed != nil && closed.IsNoun() && !closed.IsProperNoun()
}
func assertMerged(t *testing.T, input string, want string, expect bool) {
	t.Helper()
	d := doc.NewPlainEnglishCurated(input)
	tokens := d.GetTokens()
	got, ok := NewMergeableWords(mergePredicate).GetMergedWord(tokens[0], tokens[2], d.GetSource())
	if ok != expect || (ok && string(got) != want) {
		t.Fatalf("%q got %q %v, want %q %v", input, string(got), ok, want, expect)
	}
}
func TestMergesOpenCompoundNotInDict(t *testing.T)    { assertMerged(t, "note book", "notebook", true) }
func TestDoesNotMergeOpenCompoundInDict(t *testing.T) { assertMerged(t, "straight away", "", false) }
func TestDoesNotMergeInvalidCompound(t *testing.T)    { assertMerged(t, "quick fox", "", false) }
func TestMergesOpenCompound(t *testing.T)             { assertMerged(t, "front line", "frontline", true) }
func TestMergesHyphenatedCompound(t *testing.T)       { assertMerged(t, "front-line", "frontline", true) }

var goodReflexivePronouns = []string{"herself", "himself", "itself", "myself", "oneself", "ourself", "ourselves", "themselves", "thyself", "yourself", "yourselves"}
var badReflexivePronouns = []string{"hisself", "oneselves", "theirself", "theirselves"}

func assertPronounMetadata(t *testing.T, word string, good bool) {
	t.Helper()
	d := doc.NewPlainEnglishCurated(word)
	tokens := d.GetTokens()
	if len(tokens) == 0 {
		t.Fatal("No tokens in document")
	}
	k := tokens[0].Kind
	if !k.IsWord() {
		t.Fatalf("%q not a Word", word)
	}
	if good {
		if k.Metadata == nil {
			t.Fatalf("Widely accepted pronoun %q missing from dictionary", word)
		}
		if !k.Metadata.IsPronoun() || !k.Metadata.IsReflexivePronoun() {
			t.Fatalf("%q not a reflexive pronoun", word)
		}
	} else if k.Metadata != nil {
		t.Fatalf("Unaccepted/made-up pronoun %q now in dictionary", word)
	}
}
func TestGoodReflexivePronouns(t *testing.T) {
	for _, word := range goodReflexivePronouns {
		assertPronounMetadata(t, word, true)
	}
}
func TestBadReflexivePronouns(t *testing.T) {
	for _, word := range badReflexivePronouns {
		assertPronounMetadata(t, word, false)
	}
}
func TestNonPronouns(t *testing.T) {
	for _, word := range []string{"myselves", "weselves", "usself", "usselves"} {
		assertPronounMetadata(t, word, false)
	}
}
func TestEnsureStandardCtorIncludesMyself(t *testing.T) {
	assertMatchCount(t, NewReflexivePronounStandard(), "If you want something done, do it yourself.", 1)
}
func TestEnsureDefaultCtorIncludesMyself(t *testing.T) {
	assertMatchCount(t, NewReflexivePronoun(), "I wanted a reflexive pronoun module, so I wrote one myself.", 1)
}
func TestEnsureWithCommonErrorsIncludesHisself(t *testing.T) {
	assertMatchCount(t, NewReflexivePronounWithCommonErrors(), "He teached hisself English.", 1)
}
func TestEnsureStandardCtorExcludesHisself(t *testing.T) {
	assertMatchCount(t, NewReflexivePronounStandard(), "Was he pleased with hisself?", 0)
}
func TestEnsureDefaultCtorExcludesTheirself(t *testing.T) {
	assertMatchCount(t, NewReflexivePronoun(), "They look at theirself in the mirror.", 0)
}
