package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestClaimToFame_CorrectClaimForFame(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Although it is nice to go archive diving, the main claim for fame is of course the derivation of modern technologies", NewClaimToFame(), "Although it is nice to go archive diving, the main claim to fame is of course the derivation of modern technologies")
}

func TestClaimToFame_CorrectClaimsForFame(t *testing.T) {
	linttest.AssertSuggestionResult(t, "we note and dismiss offhand some of the bulimic claims for fame that have been attached to Kuhn's classic", NewClaimToFame(), "we note and dismiss offhand some of the bulimic claims to fame that have been attached to Kuhn's classic")
}

func TestClaimToFame_DontMadeClaimsForReasonsIncludingFame(t *testing.T) {
	linttest.AssertNoLints(t, "His attorney says that he made those claims for fame and money.", NewClaimToFame())
	linttest.AssertNoLints(t, "denied the allegations, saying Davis has acknowledged making claims for fame and notoriety", NewClaimToFame())
}

func TestClaimToFame_FixClaimForFameAndPron(t *testing.T) {
	linttest.AssertSuggestionResult(t, "My one claim for fame and I blew it.", NewClaimToFame(), "My one claim to fame and I blew it.")
}

func TestClaimToFame_DontFlagMakeGrandClaimsForFame(t *testing.T) {
	linttest.AssertNoLints(t, "There have always been people throughout history who are innovative and make grand claims for fame.", NewClaimToFame())
}

func TestClaimToFame_FixClaimForFameAndSo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Alan Holt has placed his claim for fame and so I've corrected this lovely image", NewClaimToFame(), "Alan Holt has placed his claim to fame and so I've corrected this lovely image")
}

func TestClaimToFame_CantFlagAmbiguousAndVerbIsAlsoAdj(t *testing.T) {
	linttest.AssertSuggestionResult(t, "VRAAQ and level criteria reinforced the students' claim for fame and enhanced the establishment of trust", NewClaimToFame(), "VRAAQ and level criteria reinforced the students' claim to fame and enhanced the establishment of trust")
}
