package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/linttest"
	"strings"
	"testing"
)

func TestSplitWords_Issue1905(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I want to try this insteadof that.", NewSplitWords(), "I want to try this instead of that.")
}

func TestSplitWords_Issue1905Rev(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I want to try thisinstead of that.", NewSplitWords(), "I want to try this instead of that.")
}

func TestSplitWords_SplitCommon(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is notnot a problem.", NewSplitWords(), "This is not not a problem.")
}

func TestSplitWords_SplitsMultipleCompoundWords(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We stared intothe darkness and kindof panicked about sortof everything.", NewSplitWords(), "We stared into the darkness and kind of panicked about sort of everything.")
}

func TestSplitWords_SplitsWordWithLongerPrefix(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The astronauts waited on the landingpad for hours.", NewSplitWords(), "The astronauts waited on the landing pad for hours.")
}

func TestSplitWords_SplitsBeforePunctuation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This was kindof, actually, hilarious.", NewSplitWords(), "This was kind of, actually, hilarious.")
}

func TestSplitWords_IgnoresKnownCompoundWords(t *testing.T) {
	linttest.AssertNoLints(t, "Someone left early.", NewSplitWords())
}

func TestSplitWords_IgnoresPrefixWithoutValidRemainder(t *testing.T) {
	linttest.AssertNoLints(t, "The monkeyxyz escaped unnoticed.", NewSplitWords())
}

func TestSplitWords_IgnoresSingleWordMisspellingWithSplitLikeHalves(t *testing.T) {
	linttest.AssertNoLints(t, "I love this extention!", NewSplitWords())
}

func TestSplitWords_Issue4130DefersHaventToSpellcheck(t *testing.T) {
	linttest.AssertNoLints(t, "They havent reviewed it yet.", NewSplitWords())
}

func TestSplitWords_Issue4130DoesNotRegressRealShortAnchors(t *testing.T) {
	linttest.AssertSuggestionResult(t, "don't seem to support symbolic links atall.", NewSplitWords(), "don't seem to support symbolic links at all.")
}

func TestSplitWords_CorrectsDoesthe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "doesthe", NewSplitWords(), "does the")
}

func TestSplitWords_CorrectsSplitwords(t *testing.T) {
	linttest.AssertSuggestionResult(t, "splitwords", NewSplitWords(), "split words")
}

func TestSplitWords_TestAtallToAtAll(t *testing.T) {
	linttest.AssertSuggestionResult(t, "don't seem to support symbolic links atall.", NewSplitWords(), "don't seem to support symbolic links at all.")
}

func TestSplitWords_TestAtallToATall(t *testing.T) {
	linttest.AssertSuggestionResult(t, "atall", NewSplitWords(), "a tall")
}

func TestSplitWords_AtallShouldSplitToATallAndAtAll(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "atall", NewSplitWords(), []string{"a tall", "at all"}, []string{})
}

func TestSplitWords_Issue2763Leaves(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I love to eat cornleaves.", NewSplitWords(), "I love to eat corn leaves.")
}

func TestSplitWords_Issue2763Husks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I love to eat cornhusks.", NewSplitWords(), "I love to eat corn husks.")
}

func TestSplitWords_Issue2763Singular(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I would love to eat a cornleaf.", NewSplitWords(), "I would love to eat a corn leaf.")
}

func TestSplitWords_ConfidentThankyouShouldBeThankYou(t *testing.T) {
	linttest.AssertLintMessage(t, "thankyou", NewSplitWords(), "`thankyou` should probably be written as `thank you`.")
}

func TestSplitWords_AllowsInformalLaughter(t *testing.T) {
	for _, source := range []string{"hah", "haha", "hahah", "hahaha", "Hahahah"} {
		linttest.AssertNoLints(t, source, NewSplitWords())
	}
}

func TestSplitWords_DoesNotSplitIff(t *testing.T) {
	linttest.AssertNoLints(t, "iff", NewSplitWords())
}

func TestSplitWords_NeverCorrectsToInvalidSingleLetterWords(t *testing.T) {
	for _, trigger := range []string{"comitted", "testc", "testh", "testb", "testq", "testx", "testg", "teste", "testj", "shes", "proc"} {
		for _, issue := range NewSplitWords().Lint(doc.NewPlainEnglishCurated(trigger)) {
			for _, suggestion := range issue.Suggestions {
				if suggestion.IsReplaceWith() {
					for _, word := range strings.Split(string(suggestion.Replacement), " ") {
						if len([]rune(word)) == 1 && strings.ContainsRune("cdthbxejs", []rune(word)[0]) {
							t.Fatalf("Encountered bad output %q", word)
						}
					}
				}
			}
		}
	}
}
