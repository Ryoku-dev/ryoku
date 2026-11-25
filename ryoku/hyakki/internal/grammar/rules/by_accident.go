package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type ByAccident struct{ expression expr.Expr }

func NewByAccident() *ByAccident {
	return &ByAccident{expression: expr.NewSequenceExprAco("on").TWs().ThenOptional(expr.NewSequenceExprWordSet([]string{"complete", "happy", "literal", "mere", "pure", "sheer", "total"}).TWs()).TAco("accident")}
}
func (r *ByAccident) Expr() expr.Expr                  { return r.expression }
func (r *ByAccident) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*ByAccident) Description() string {
	return "Incorrect preposition: `by accident` is the idiomatic expression."
}
func (r *ByAccident) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	span := toks[0].Span
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Usage
	l.Message = "Did you mean `by accident`?"
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("by", span.GetContent(src))}
	return &l
}
