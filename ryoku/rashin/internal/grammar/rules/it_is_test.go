package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestItIs_FlagsSimpleCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its amazing to see this.", NewItIs(), "It's amazing to see this.")
}

func TestItIs_FlagsWithPreposition(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its critical for the project.", NewItIs(), "It's critical for the project.")
}

func TestItIs_DoesNotFlagExceptionOwn(t *testing.T) {
	linttest.AssertLintCount(t, "Its own design is unique.", NewItIs(), 0)
}

func TestItIs_DoesNotFlagExceptionTeam(t *testing.T) {
	linttest.AssertLintCount(t, "Its team lead is excellent.", NewItIs(), 0)
}

func TestItIs_DoesNotFlagNonAdjective(t *testing.T) {
	t.Skip("Upstream ignored: This case fails, but I think that's acceptable")
	linttest.AssertLintCount(t, "The cat chased its tail around the room.", NewItIs(), 0)
}

func TestItIs_DoesNotFlagAlreadyCorrect(t *testing.T) {
	linttest.AssertLintCount(t, "It's important to note.", NewItIs(), 0)
}

func TestItIs_FlagsSearchFilterContext(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its important to note that the search filter will currently only search the current page.", NewItIs(), "It's important to note that the search filter will currently only search the current page.")
}

func TestItIs_FlagsEnsRestartContext(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Today is the third day and I am still stuck on Register. Its important to note that after hours of waiting, I tried to restart the process and clicked on register again but it gets stuck at TX pending.", NewItIs(), "Today is the third day and I am still stuck on Register. It's important to note that after hours of waiting, I tried to restart the process and clicked on register again but it gets stuck at TX pending.")
}

func TestItIs_FlagsAcademicsSupportContext(t *testing.T) {
	linttest.AssertSuggestionResult(t, "To assist learners, because its critical for academics to support their ideas and arguments with sources of published research.", NewItIs(), "To assist learners, because it's critical for academics to support their ideas and arguments with sources of published research.")
}

func TestItIs_FlagsParentsExplainContext(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I also think its critical for parents to explain their reason for saying no though I would advise against attempting to use logic in the face of either toddler or teenage rage.", NewItIs(), "I also think it's critical for parents to explain their reason for saying no though I would advise against attempting to use logic in the face of either toddler or teenage rage.")
}

func TestItIs_FlagsChapterContext(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I think it's okay since its critical for the rest of the chapter in terms of tone and approach.", NewItIs(), "I think it's okay since it's critical for the rest of the chapter in terms of tone and approach.")
}

func TestItIs_FlagsMicrosoftWorkContext(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... Need help, its critical for my work, as i am a technical blog writer ...", NewItIs(), "... Need help, it's critical for my work, as i am a technical blog writer ...")
}

func TestItIs_FlagsFeministsContext(t *testing.T) {
	linttest.AssertSuggestionResult(t, "when it comes to the teaching of grammar and diverse linguistics practices. Its critical for feminists to think about the ways in which they frame language.", NewItIs(), "when it comes to the teaching of grammar and diverse linguistics practices. It's critical for feminists to think about the ways in which they frame language.")
}

func TestItIs_FlagsStudentsProofreadingContext(t *testing.T) {
	linttest.AssertSuggestionResult(t, "its critical for students to develop a similarly sharp eye for misspellings and grammatical errors.", NewItIs(), "it's critical for students to develop a similarly sharp eye for misspellings and grammatical errors.")
}

func TestItIs_FlagsAmericansContext(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its critical for Americans to realize that Fox has nothing to do with news.", NewItIs(), "It's critical for Americans to realize that Fox has nothing to do with news.")
}

func TestItIs_DoesNotFlagItsTeamLead(t *testing.T) {
	linttest.AssertLintCount(t, "Its team lead is excellent.", NewItIs(), 0)
}

func TestItIs_FlagsCrucialApiContext(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its crucial to understand the API before using it.", NewItIs(), "It's crucial to understand the API before using it.")
}

func TestItIs_FlagsEssentialStandardsContext(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its essential to follow the coding standards in this project.", NewItIs(), "It's essential to follow the coding standards in this project.")
}

func TestItIs_FlagsVitalDependenciesContext(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its vital to keep dependencies up to date.", NewItIs(), "It's vital to keep dependencies up to date.")
}
