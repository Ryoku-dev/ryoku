package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestPluralWrongWordOfPhrase_CorrectsLineOfCodes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "desktop application used to estimate the line of codes to certain software application", NewPluralWrongWordOfPhrase(), "desktop application used to estimate the lines of code to certain software application")
}

func TestPluralWrongWordOfPhrase_CorrectsLineOfCodesTitleCase(t *testing.T) {
	t.Skip("Upstream ignored: Wrong letters capitalized due to how `Suggstion::replace_with_match_case` works by index.")
	linttest.AssertSuggestionResult(t, "A simple tool for Line Of Codes (LOC) calculation.", NewPluralWrongWordOfPhrase(), "A simple tool for Lines Of Code (LOC) calculation.")
}

func TestPluralWrongWordOfPhrase_CorrectsLinesOfCodes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I myself don't have something against giving users the ability to show the lines of codes they wrote.", NewPluralWrongWordOfPhrase(), "I myself don't have something against giving users the ability to show the lines of code they wrote.")
}

func TestPluralWrongWordOfPhrase_CorrectsPartOfSpeeches(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The part of speeches (POS) or as follows:", NewPluralWrongWordOfPhrase(), "The parts of speech (POS) or as follows:")
}

func TestPluralWrongWordOfPhrase_CorrectsPartsOfSpeeches(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It can connect different parts of speeches e.g noun to adjective, adjective to adverb, noun to verb etc.", NewPluralWrongWordOfPhrase(), "It can connect different parts of speech e.g noun to adjective, adjective to adverb, noun to verb etc.")
}

func TestPluralWrongWordOfPhrase_CorrectsPointOfViews(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This will produce a huge amount of raw data, representing the region in multiple point of views.", NewPluralWrongWordOfPhrase(), "This will produce a huge amount of raw data, representing the region in multiple points of view.")
}

func TestPluralWrongWordOfPhrase_CorrectsPointsOfViews(t *testing.T) {
	linttest.AssertSuggestionResult(t, "log events, places, moods and self-reflect from various points of views", NewPluralWrongWordOfPhrase(), "log events, places, moods and self-reflect from various points of view")
}

func TestPluralWrongWordOfPhrase_CorrectRuleOfThumbs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Thanks. 0.2 is just from my rule of thumbs.", NewPluralWrongWordOfPhrase(), "Thanks. 0.2 is just from my rules of thumb.")
}

func TestPluralWrongWordOfPhrase_CorrectRulesOfThumbs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But as rules of thumbs, what is said in config file should be respected whatever parameter (field or directory) is passed to php-cs-fixer.phar.", NewPluralWrongWordOfPhrase(), "But as rules of thumb, what is said in config file should be respected whatever parameter (field or directory) is passed to php-cs-fixer.phar.")
}

func TestPluralWrongWordOfPhrase_CorrectRulesOfThumbsHyphenated(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Add rule-of-thumbs for basic metrics, like \"Spill more than 1GB is a red flag\".", NewPluralWrongWordOfPhrase(), "Add rules-of-thumb for basic metrics, like \"Spill more than 1GB is a red flag\".")
}

func TestPluralWrongWordOfPhrase_CorrectBodyInWhites1(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Normally, when they manufacture these body in whites, they would spot weld a lot of the components on.", NewPluralWrongWordOfPhrase(), "Normally, when they manufacture these bodies in white, they would spot weld a lot of the components on.")
}

func TestPluralWrongWordOfPhrase_CorrectBodyInWhites2(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm not sure, but just having seen a lot of body in whites, I know normally they try to spot weld it.", NewPluralWrongWordOfPhrase(), "I'm not sure, but just having seen a lot of bodies in white, I know normally they try to spot weld it.")
}

func TestPluralWrongWordOfPhrase_CorrectFlashInThePans(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I wish they do more flash in the pans, like the suggestions they do on ERB2 could be such a good way to see if these suggestions are worthy.", NewPluralWrongWordOfPhrase(), "I wish they do more flashes in the pan, like the suggestions they do on ERB2 could be such a good way to see if these suggestions are worthy.")
}

func TestPluralWrongWordOfPhrase_CorrectFlashInThePansHyphenated(t *testing.T) {
	linttest.AssertSuggestionResult(t, "what makes 'Super Hexagon' rise above other nostalgic flash-in-the-pans is that there is a game to be learned here", NewPluralWrongWordOfPhrase(), "what makes 'Super Hexagon' rise above other nostalgic flashes-in-the-pan is that there is a game to be learned here")
}

func TestPluralWrongWordOfPhrase_CorrectFlashesInThePans(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Who are some of the biggest flashes in the pans in wrestling history?", NewPluralWrongWordOfPhrase(), "Who are some of the biggest flashes in the pan in wrestling history?")
}

func TestPluralWrongWordOfPhrase_CorrectProofOfConcepts(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I wrote about my love of throwaway prototypes; those little proof-of-concepts that exist purely to get an idea out of your head", NewPluralWrongWordOfPhrase(), "I wrote about my love of throwaway prototypes; those little proofs-of-concept that exist purely to get an idea out of your head")
}

func TestPluralWrongWordOfPhrase_CorrectMomentsOfTruths(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He does seem like a good-ish guy at the most crucial moments of truths.", NewPluralWrongWordOfPhrase(), "He does seem like a good-ish guy at the most crucial moments of truth.")
}

func TestPluralWrongWordOfPhrase_CorrectMomentOfTruths(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Being aware of the moment of truths and prepared to interact with your potential customers", NewPluralWrongWordOfPhrase(), "Being aware of the moments of truth and prepared to interact with your potential customers")
}
