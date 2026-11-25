package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type ArriveTo struct{ expression expr.Expr }

func NewArriveTo() *ArriveTo {
	return &ArriveTo{expression: expr.ButNot(expr.NewSequenceExprWordSet([]string{"arrive", "arrived", "arrives", "arriving"}).TWs().TAco("to"), expr.NewSequenceExprAnything().TAny().TAny().TAny().ThenVerbLemma())}
}
func (r *ArriveTo) Expr() expr.Expr                  { return r.expression }
func (r *ArriveTo) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*ArriveTo) Description() string                { return "Corrects `arrive to` to `arrive at` or `arrive in`." }
func (r *ArriveTo) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span := t[2].Span
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Usage
	l.Suggestions = lint.ToReplaceSuggestions([]string{"at", "in"}, s[span.Start:span.End])
	l.Message = "If the noun is a destination, use 'at' or 'in' instead of 'to'."
	return &l
}
