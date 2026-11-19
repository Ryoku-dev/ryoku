package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type OnFloor struct{ expression expr.Expr }

func NewOnFloor() *OnFloor {
	floor := expr.NewSequenceExprWordSet([]string{"in", "at"}).TWs().TAco("the").TWs().TAny().TWs().TAco("floor")
	exceptions := expr.NewLongestMatchOf([]expr.Expr{expr.NewSequenceExprWordSet([]string{"look", "looking", "looks", "looked"}).TWs().TAco("up"), expr.NewSequenceExprWordSet([]string{"stop", "stopping", "stops", "stopped"})})
	return &OnFloor{expr.NewLongestMatchOf([]expr.Expr{floor, expr.NewSequenceExprWith(exceptions).TWs().Then(floor)})}
}
func (r *OnFloor) Expr() expr.Expr                  { return r.expression }
func (r *OnFloor) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*OnFloor) Description() string {
	return "This rule identifies incorrect uses of the prepositions `in` or `at` when referring to locations inside a building and recommends using `on the floor` instead."
}
func (r *OnFloor) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	w := t[0]
	incorrect := w.GetStr(s)
	if !strings.EqualFold(incorrect, "in") && !strings.EqualFold(incorrect, "at") {
		return nil
	}
	return &lint.Lint{Span: w.Span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("on", w.GetCh(s))}, Message: fmt.Sprintf("Corrects `%s` to `on` when talking about position inside a building", incorrect), Priority: 63}
}
