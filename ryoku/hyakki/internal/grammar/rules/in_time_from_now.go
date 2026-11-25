package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type InTimeFromNow struct{ expression expr.Expr }

func NewInTimeFromNow() *InTimeFromNow {
	return &InTimeFromNow{expression: expr.NewSequenceExprAco("in").TWs().ThenOptional(expr.NewSequenceExprWordSet([]string{"about", "almost", "approximately", "around", "circa", "exactly", "just", "maybe", "nearly", "only", "perhaps", "precisely", "probably", "roughly"}).TWs()).Then(expr.NewDurationExpr()).TWs().TAco("from").TWs().TAco("now")}
}
func (r *InTimeFromNow) Expr() expr.Expr                  { return r.expression }
func (r *InTimeFromNow) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *InTimeFromNow) Description() string {
	return "Checks for redundant use of `in` before [period of time] together with `from now` after it."
}
func (r *InTimeFromNow) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span, ok := doc.Span(t)
	if !ok {
		return nil
	}
	withoutIn, ok := doc.GetCh(t[2:], s)
	if !ok {
		return nil
	}
	withoutFromNow, ok := doc.GetCh(t[:len(t)-4], s)
	if !ok {
		return nil
	}
	template := s[span.Start:span.End]
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Redundancy
	result.Message = "Avoid redundancy by using either `in [period of time]` or `[period of time] from now`, but not both together."
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCase(withoutIn, template), lint.ReplaceWithMatchCase(withoutFromNow, template)}
	return &result
}
