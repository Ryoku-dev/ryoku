package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type AWaysToGo struct{ expression expr.Expr }

func NewAWaysToGo() *AWaysToGo {
	core := expr.NewSequenceExprWordSet([]string{"have", "had", "has", "having"}).TWs().ThenWordSeq([]string{"ways", "to", "go"})
	epilog := expr.NewFirstMatchOf([]expr.Expr{expr.NewAnchorEnd(), expr.NewSequenceExprWhitespace().ThenAnyOf([]expr.Expr{expr.NewSequenceExprWordSet([]string{"before", "but", "though"}), expr.NewSequenceExprWordSeq([]string{"in", "order", "to"}), expr.NewSequenceExprWordSeq([]string{"to", "be"})})})
	return &AWaysToGo{expression: expr.NewFirstMatchOf([]expr.Expr{expr.NewSequenceExprAco("still").TWs().Then(core), expr.NewSequenceExprWith(core).Then(epilog)})}
}
func (r *AWaysToGo) Expr() expr.Expr                  { return r.expression }
func (r *AWaysToGo) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *AWaysToGo) Description() string {
	return "Corrects the idiom `a ways to go` when the indefinite article is missing."
}
func (r *AWaysToGo) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	idx, ok := lint.FindTheOnlyTokenIndexMatching(toks, src, func(t *doc.Token, s []rune) bool { return text.EqStr(t.GetCh(s), "ways") })
	if !ok || idx < 2 {
		return nil
	}
	l := lint.NewLint()
	l.Span = toks[idx-2].Span
	l.LintKind = lint.Usage
	l.Suggestions = []lint.Suggestion{lint.InsertAfter([]rune(" a"))}
	l.Message = "This idiom requires the word `a`."
	return &l
}
