package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ExpandIme struct{ expression expr.Expr }

func NewExpandIme() *ExpandIme {
	return &ExpandIme{expression: expr.NewSequenceExprWith(expr.NewAnchorStart()).TAco("ime")}
}
func (r *ExpandIme) Expr() expr.Expr { return r.expression }
func (r *ExpandIme) Description() string {
	return "Expands the abbreviation `ime` to the full phrase `in my experience` for clarity."
}
func (r *ExpandIme) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ExpandIme) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	span := text.Span{Start: t[0].Span.Start, End: t[len(t)-1].Span.End}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Miscellaneous
	result.Suggestions = []lint.Suggestion{lint.ReplaceWith([]rune("In my experience"))}
	result.Message = "Use `in my experience` instead of `ime`, if that's what you mean."
	result.Priority = 127
	return &result
}
