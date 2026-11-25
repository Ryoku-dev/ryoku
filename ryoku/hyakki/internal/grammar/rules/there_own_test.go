package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestThereOwn_CorrectsThereOwn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Users can split data on there own topics.", NewThereOwn(), "Users can split data on their own topics.")
}

func TestThereOwn_CorrectsTheyreOwn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Everybody has they're own preferences.", NewThereOwn(), "Everybody has their own preferences.")
}

func TestThereOwn_CorrectsTheyreNoApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It would be helpful for people building theyre own rockets.", NewThereOwn(), "It would be helpful for people building their own rockets.")
}

func TestThereOwn_PreservesCapitalization(t *testing.T) {
	linttest.AssertSuggestionResult(t, "There own connection pool must be configured.", NewThereOwn(), "Their own connection pool must be configured.")
}

func TestThereOwn_DoesNotFlagCorrectTheirOwn(t *testing.T) {
	linttest.AssertLintCount(t, "They manage their own servers.", NewThereOwn(), 0)
}

func TestThereOwn_DoesNotFlagThereWithoutOwn(t *testing.T) {
	linttest.AssertLintCount(t, "Put the chairs over there by the window.", NewThereOwn(), 0)
}

func TestThereOwn_DoesNotFlagVerbOwnAfterNoun(t *testing.T) {
	linttest.AssertLintCount(t, "People there own nice cars.", NewThereOwn(), 0)
}

func TestThereOwn_DoesNotFlagVerbOwnWithDeterminer(t *testing.T) {
	linttest.AssertLintCount(t, "Companies there own the property.", NewThereOwn(), 0)
}

func TestThereOwn_Issue3276CustomiseDefaultLvl(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Provide user the option in setting to customise there own default Effort lvl.", NewThereOwn(), "Provide user the option in setting to customise their own default Effort lvl.")
}

func TestThereOwn_Issue3276ModulesCFiles(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Allow for modules to provide there own c files.", NewThereOwn(), "Allow for modules to provide their own c files.")
}

func TestThereOwn_Issue3276ScriptsCreateConnection(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have a number of scripts that all create there own connection.", NewThereOwn(), "I have a number of scripts that all create their own connection.")
}

func TestThereOwn_Issue3276SilentAppstoreUpdates(t *testing.T) {
	linttest.AssertSuggestionResult(t, "allowing 3rd party appstores to silently update apps installed by theyre own.", NewThereOwn(), "allowing 3rd party appstores to silently update apps installed by their own.")
}

func TestThereOwn_Issue3276TheyreOwnCommand(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But because they are they're own command they have their own runtime.", NewThereOwn(), "But because they are their own command they have their own runtime.")
}

func TestThereOwn_Issue3276TheyreOwnLibrary(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Components I'm working on that will eventually become they're own library.", NewThereOwn(), "Components I'm working on that will eventually become their own library.")
}

func TestThereOwn_Issue3276FalsePositiveExpensiveCars(t *testing.T) {
	linttest.AssertLintCount(t, "People there own expensive cars.", NewThereOwn(), 0)
}

func TestThereOwn_SentenceInitialThereOwnStillFlagged(t *testing.T) {
	linttest.AssertSuggestionResult(t, "There own employees disagreed with the policy.", NewThereOwn(), "Their own employees disagreed with the policy.")
}
