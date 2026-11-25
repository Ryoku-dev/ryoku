package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type LeavingInDroves struct{ expression expr.Expr }

func NewLeavingInDroves() *LeavingInDroves {
	return &LeavingInDroves{expression: expr.NewSequenceExprWordSet([]string{"leave", "leaves", "leaving", "left"}).TWs().TAco("in").TWs().TAco("drones")}
}
func (r *LeavingInDroves) Expr() expr.Expr                  { return r.expression }
func (r *LeavingInDroves) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*LeavingInDroves) Description() string {
	return "Corrects `leaving in drones` to `leaving in droves`."
}
func (r *LeavingInDroves) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	tok := t[len(t)-1]
	l := lint.NewLint()
	l.Span = tok.Span
	l.LintKind = lint.Eggcorn
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("droves", tok.GetCh(s))}
	l.Message = "`Drones` is an eggcorn. The correct word is `droves`."
	return &l
}
