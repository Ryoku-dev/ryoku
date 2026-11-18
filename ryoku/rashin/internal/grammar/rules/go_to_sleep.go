package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type GoToSleep struct{ expression expr.Expr }

func NewGoToSleep() *GoToSleep {
	return &GoToSleep{expression: expr.ButNot(expr.NewSequenceExprWordSet([]string{"go", "goes", "going", "gone", "went"}).TWs().ThenWordSeq([]string{"into", "sleep"}), expr.NewSequenceExprAnything().TAny().TAny().TAny().TAny().ThenAnyOf([]expr.Expr{expr.NewSequenceExprWhitespace().ThenAnyOf([]expr.Expr{expr.AsExpr(expr.NewWordSet([]string{"mode", "state"})), expr.NewSequenceExpr().ThenAdjective().TWs().TSet([]string{"mode", "state"})}), expr.NewSequenceExpr().ThenSlash().TSet([]string{"hibernate", "hibernation", "suspend"})}))}
}
func (r *GoToSleep) Expr() expr.Expr                  { return r.expression }
func (r *GoToSleep) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *GoToSleep) Description() string              { return "Corrects `go into sleep` to `go to sleep`." }
func (r *GoToSleep) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 3 {
		return nil
	}
	span := toks[2].Span
	return &lint.Lint{Span: span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("to", span.GetContent(src))}, Message: "Use `go to sleep` instead of `go into sleep`.", Priority: 127}
}
