package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type DoubleModal struct{ expression expr.Expr }

func NewDoubleModal() *DoubleModal {
	return &DoubleModal{expression: expr.NewSequenceExprWith(expr.NewModalVerb()).TWs().Then(expr.NewModalVerb())}
}
func (r *DoubleModal) Expr() expr.Expr { return r.expression }
func (r *DoubleModal) Description() string {
	return "Two modal verbs in a row are rarely grammatical; remove one of them."
}
func (r *DoubleModal) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *DoubleModal) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	span := text.Span{Start: t[0].Span.Start, End: t[len(t)-1].Span.End}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Miscellaneous
	result.Suggestions = []lint.Suggestion{lint.ReplaceWith(t[0].GetCh(s)), lint.ReplaceWith(t[len(t)-1].GetCh(s))}
	result.Message = "Two modal verbs in a row are rarely grammatical; remove one."
	result.Priority = 31
	return &result
}
