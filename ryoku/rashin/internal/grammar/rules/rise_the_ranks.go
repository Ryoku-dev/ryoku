package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type RiseTheRanks struct{ expression expr.Expr }

func NewRiseTheRanks() *RiseTheRanks {
	return &RiseTheRanks{expression: expr.NewSequenceExprWordSet([]string{"rise", "risen", "rising", "rose", "rised"}).TWs().TAco("the").TWs().TAco("ranks")}
}
func (r *RiseTheRanks) Expr() expr.Expr { return r.expression }
func (r *RiseTheRanks) Description() string {
	return "Corrects the nonstandard phrase `rise the ranks` to the standard `rise through the ranks` or `rise from the ranks`"
}
func (r *RiseTheRanks) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *RiseTheRanks) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return &lint.Lint{Span: t[0].Span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.InsertAfter([]rune(" through")), lint.InsertAfter([]rune(" from"))}, Message: "Use either the modern standard 'rise through the ranks' or the traditional 'rise from the ranks'. 'Rise the ranks' is nonstandard.", Priority: 127}
}
