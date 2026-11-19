package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestThePointFor_FixIs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "What is the point for this check?", NewThePointFor(), "What is the point of this check?")
}

func TestThePointFor_FixThatsThePointNoApostrophe(t *testing.T) {
	t.Skip("'that' is disabled to avoid false positives until the heuristics can be improved")
	linttest.AssertSuggestionResult(t, "You should also keep an eye on the issue list because thats the point for being public", NewThePointFor(), "You should also keep an eye on the issue list because thats the point of being public")
}

func TestThePointFor_FixWasThePoint(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But avoiding to learn qraphql to find out the proper query was the point for asking for this convenience command.", NewThePointFor(), "But avoiding to learn qraphql to find out the proper query was the point of asking for this convenience command.")
}

func TestThePointFor_FixWhatsThePoint(t *testing.T) {
	linttest.AssertSuggestionResult(t, "What's the point for IRepository?", NewThePointFor(), "What's the point of IRepository?")
}

func TestThePointFor_FixWhatsThePointNoApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "## I dont get RSS. Whats the point for using a reader like feedly, if the articles open in their native website", NewThePointFor(), "## I dont get RSS. Whats the point of using a reader like feedly, if the articles open in their native website")
}

func TestThePointFor_AvoidFlaggingThePointForWhich(t *testing.T) {
	linttest.AssertNoLints(t, "p0 is the point for which we want to find the closest point to the line", NewThePointFor())
}
