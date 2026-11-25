package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type WellEducated struct{ expression expr.Expr }

func NewWellEducated() *WellEducated {
	e := expr.NewSequenceExprAnyOf([]expr.Expr{expr.AsExpr(expr.NewWordSet([]string{"good-educated"})), expr.NewSequenceExpr().TAco("good").ThenOptionalWhitespace().ThenHyphen().ThenOptionalWhitespace().TAco("educated")})
	return &WellEducated{expression: e}
}
func (r *WellEducated) Expr() expr.Expr { return r.expression }
func (r *WellEducated) Description() string {
	return "Replaces `good-educated` with the accepted compound `well-educated`."
}
func (r *WellEducated) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *WellEducated) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span, ok := doc.Span(t)
	if !ok {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Miscellaneous
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("well-educated", span.GetContent(s))}
	l.Message = "Prefer `well-educated` for this compound."
	l.Priority = 35
	return &l
}
