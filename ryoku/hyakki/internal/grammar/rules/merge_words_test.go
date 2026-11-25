package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestMergeWords_Clean(t *testing.T) {
	linttest.AssertLintCount(t, "When referring to the political party, make sure to treat them as a proper noun.", NewMergeWords(), 0)
}

func TestMergeWords_Heretofore(t *testing.T) {
	linttest.AssertLintCount(t, "This is a her etofore unseen problem.", NewMergeWords(), 1)
}

func TestMergeWords_Therefore(t *testing.T) {
	linttest.AssertLintCount(t, "The refore", NewMergeWords(), 1)
}

func TestMergeWords_ThatIsContraction(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That s", NewMergeWords(), "That's")
}

func TestMergeWords_AllowsIssue722(t *testing.T) {
	linttest.AssertLintCount(t, "Leaving S and K alone.", NewMergeWords(), 0)
	linttest.AssertLintCount(t, "Similarly an S with a line.", NewMergeWords(), 0)
}

func TestMergeWords_ChecksWeirdly(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is wrapped around the line weirdl y.", NewMergeWords(), "This is wrapped around the line weirdly.")
}
