package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type ThriveOn struct{ expression expr.Expr }

func NewThriveOn() *ThriveOn {
	return &ThriveOn{expression: expr.NewSequenceExprWordSet([]string{"thrive", "thrived", "thrives", "thriving"}).TWs().TAco("off").ThenOptional(expr.NewSequenceExprWhitespace().TAco("of"))}
}
func (r *ThriveOn) Expr() expr.Expr                  { return r.expression }
func (r *ThriveOn) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ThriveOn) Description() string {
	return "Corrects `thrive off` and `thrive off of` to `thrive on`."
}
func (r *ThriveOn) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks[2:])
	if !ok {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Style
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("on", span.GetContent(src))}
	l.Message = "Consider using `thrive on` instead of `thrive off` or `thrive off of`."
	return &l
}
