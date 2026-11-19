package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type FishNorFowl struct{ expression expr.Expr }

func NewFishNorFowl() *FishNorFowl {
	return &FishNorFowl{expression: expr.NewSequenceExprWordSet([]string{"neither", "niether"}).TWs().ThenWordSeq([]string{"fish", "nor"}).TWs().TSet([]string{"foul", "bird"})}
}
func (r *FishNorFowl) Expr() expr.Expr { return r.expression }
func (r *FishNorFowl) Description() string {
	return "Corrects `neither fish nor foul` and `neither fish nor bird` to `neither fish nor fowl`."
}
func (r *FishNorFowl) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *FishNorFowl) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	kind := lint.Nonstandard
	msg := "The standard idiom is `neither fish nor fowl`."
	if text.StartsWithIgnoreASCIICaseStr(t[len(t)-1].GetCh(s), "f") {
		kind = lint.Spelling
		msg = "The correct spelling is `fowl`, meaning `bird`. `Foul` is an adjective meaning `dirty` or `offensive`."
	}
	result := lint.NewLint()
	result.Span = t[len(t)-1].Span
	result.LintKind = kind
	result.Message = msg
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("fowl", result.Span.GetContent(s))}
	return &result
}
