package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
	"testing"
)

func TestWrongNegative_WrongNegativeWinsOverSpellCheck(t *testing.T) {
	dictionary := dict.Curated()
	group := lint.NewLintGroup()
	group.Add("SpellCheck", NewSpellCheck(dictionary, dict.American))
	group.AddChunkExprLinter("WrongNegative", NewWrongNegative(dictionary))
	group.Config.SetRuleEnabled("SpellCheck", true)
	group.Config.SetRuleEnabled("WrongNegative", true)
	document := doc.NewPlainEnglishCurated("unvisible")
	lints := lint.RemoveOverlaps(group.Lint(document))
	if len(lints) != 1 {
		t.Fatalf("got %d lints, want 1", len(lints))
	}
	if lints[0].Message != "Could this be the negative word you intended?" {
		t.Fatalf("got message %q", lints[0].Message)
	}
}
