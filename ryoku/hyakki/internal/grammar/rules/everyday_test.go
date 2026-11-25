package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestEveryday_DontFlagLoneAdjective(t *testing.T) {
	linttest.AssertLintCount(t, "everyday", NewEveryday(), 0)
}

func TestEveryday_DontFlagLoneAdverb(t *testing.T) {
	linttest.AssertLintCount(t, "every day", NewEveryday(), 0)
}

func TestEveryday_CorrectAdjectiveAtEndOfChunk(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is something I do everyday.", NewEveryday(), "This is something I do every day.")
}

func TestEveryday_CorrectAdverbAfterArticleBeforeNoun(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's nothing special, just an every day thing.", NewEveryday(), "It's nothing special, just an everyday thing.")
}

func TestEveryday_CorrectAdjectiveWithoutFollowingNoun(t *testing.T) {
	t.Skip("Can't yet match end-of-chunk after it. Adjective before is legit for both adjective and adverb.")
	linttest.AssertSuggestionResult(t, "Some git commands used everyday", NewEveryday(), "Some git commands used every day")
}

func TestEveryday_DontFlagEverydayAdjectiveBeforeDev(t *testing.T) {
	linttest.AssertLintCount(t, "At everyday dev, engineering isn't just a job - it's our passion.", NewEveryday(), 0)
}

func TestEveryday_DontFlagEverydayAdjectiveBeforePresentParticiple(t *testing.T) {
	linttest.AssertLintCount(t, "Everyday coding projects.", NewEveryday(), 0)
}

func TestEveryday_DontFlagEverydayAdjectiveBeforePluralNoun(t *testing.T) {
	linttest.AssertLintCount(t, "Exploring Everyday Things with R and Ruby", NewEveryday(), 0)
}

func TestEveryday_CorrectEverydayAtEndOfSentenceAfterPastVerb(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Trying to write about what I learned everyday.", NewEveryday(), "Trying to write about what I learned every day.")
}

func TestEveryday_DontFlagEveryDayAtStartOfSentenceBeforeComma(t *testing.T) {
	linttest.AssertLintCount(t, "Every day, a new concept or improvement will be shared", NewEveryday(), 0)
}

func TestEveryday_DontFlagEveryDayAtStartOfSentenceBeforeCopula(t *testing.T) {
	linttest.AssertLintCount(t, "Every day is worth remembering...", NewEveryday(), 0)
}

func TestEveryday_DontFlagEveryDayAtEndOfSentenceAfterNoun(t *testing.T) {
	linttest.AssertLintCount(t, "You learn new stuff every day.", NewEveryday(), 0)
}

func TestEveryday_DontFlagEveryDayAfterNounBeforeConjunction(t *testing.T) {
	linttest.AssertLintCount(t, "Pick a different test item every day and confirm it is present.", NewEveryday(), 0)
}

func TestEveryday_CorrectEveryDayAfterArticle(t *testing.T) {
	t.Skip("replace_with_match_case_str converts to EveryDay instead of Everyday")
	linttest.AssertSuggestionResult(t, "The Every Day Calendar with Dark Mode", NewEveryday(), "The Everyday Calendar with Dark Mode")
}

func TestEveryday_DontFlagEverydayBeforeUnknownWord(t *testing.T) {
	linttest.AssertLintCount(t, "It's just a normal everyday splorg.", NewEveryday(), 0)
}

func TestEveryday_DontFlagEveryDayAtEndOfChunkAfterAdverb(t *testing.T) {
	linttest.AssertLintCount(t, "I use the same amount of energy basically every day", NewEveryday(), 0)
}

func TestEveryday_DontFlagEveryDayAfterVerbBeforeIf(t *testing.T) {
	linttest.AssertLintCount(t, "This would happen every day if left alone.", NewEveryday(), 0)
}

func TestEveryday_DontFlagEveryDayAfterNounBeforePreposition(t *testing.T) {
	linttest.AssertLintCount(t, "An animal can do training and inference every day of its existence until the day of its death.", NewEveryday(), 0)
}

func TestEveryday_DontFlagEveryDayAfterTime(t *testing.T) {
	linttest.AssertLintCount(t, "Can I take a picture at 12:00 every day?", NewEveryday(), 0)
}

func TestEveryday_DontFlagEveryDayAtStartOfChunkBeforeNp(t *testing.T) {
	linttest.AssertLintCount(t, "Every day the application crashes several times on macOS Sequoia version 15.3", NewEveryday(), 0)
}

func TestEveryday_FixEverydayAndEveryDayUsedWrongly(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Each and everyday you ought to strive to learn something that is not an every day thing.", NewEveryday(), "Each and every day you ought to strive to learn something that is not an everyday thing.")
}

func TestEveryday_FixRedditWhyDoesEveryday(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Why does everyday feel the same?", NewEveryday(), "Why does every day feel the same?")
}

func TestEveryday_FixRedditEverydayIsGoingTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... everyday is going to be a good day that's just the way it is!", NewEveryday(), "... every day is going to be a good day that's just the way it is!")
}

func TestEveryday_FixRedditDrawEveryday(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Do you actually improve if you draw everyday?", NewEveryday(), "Do you actually improve if you draw every day?")
}

func TestEveryday_FixRedditTwoBadOutOfThree(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Yes you can jog everyday, not a personal best every day, but a steady pace run everyday.", NewEveryday(), "Yes you can jog every day, not a personal best every day, but a steady pace run every day.")
}

func TestEveryday_FixRedditEveryDayRoutine(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Habit stacking - stacking the small skill with something that's already worked into my every day routine.", NewEveryday(), "Habit stacking - stacking the small skill with something that's already worked into my everyday routine.")
}

func TestEveryday_FixStackoverflowEveryDayThings(t *testing.T) {
	linttest.AssertSuggestionResult(t, "However, the message goes far beyond every day things.", NewEveryday(), "However, the message goes far beyond everyday things.")
}

func TestEveryday_FixRedditEverydayIsSame(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Everyday is exactly the same", NewEveryday(), "Every day is exactly the same")
}

func TestEveryday_FixMediumLittleBitEveryday(t *testing.T) {
	t.Skip("doesn't work yet because title case demands 'Every Day' but we get 'Every day'")
	linttest.AssertSuggestionResult(t, "Does Learning A Little Bit Everyday Actually Work?", NewEveryday(), "Does Learning A Little Bit Every Day Actually Work?")
}

func TestEveryday_FixStackexchangeUseEveryday(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We use this everyday without noticing, but we hate it when ...", NewEveryday(), "We use this every day without noticing, but we hate it when ...")
}

func TestEveryday_FixGithubWhatILearnedEveryday(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Trying to write about what I learned everyday.", NewEveryday(), "Trying to write about what I learned every day.")
}

func TestEveryday_FixMediumOneBadOutOfThree(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Even inside a routine, everyday we adapt to changes and challenges ... We are not the same person every day, but every day we are ourselves…", NewEveryday(), "Even inside a routine, every day we adapt to changes and challenges ... We are not the same person every day, but every day we are ourselves…")
}

func TestEveryday_FixMediumDoingSomethingEveryday(t *testing.T) {
	linttest.AssertSuggestionResult(t, "There was nothing wrong with my braincells processing the concepts of doing something everyday and ...", NewEveryday(), "There was nothing wrong with my braincells processing the concepts of doing something every day and ...")
}

func TestEveryday_FixMediumAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "MEET SOMEONE NEW EVERYDAY.", NewEveryday(), "MEET SOMEONE NEW EVERY DAY.")
}

func TestEveryday_DontFlagEveryDaySingularNoun2020(t *testing.T) {
	linttest.AssertNoLints(t, "50 requests per day, every day free.", NewEveryday())
}
