package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type ASomeTime struct{ expression expr.Expr }

func NewASomeTime() *ASomeTime {
	return &ASomeTime{expression: expr.NewSequenceExprWordSeq([]string{"a", "some"}).TWs().ThenAnyOf([]expr.Expr{expr.NewWord("time"), expr.NewTimeUnitExprPluralsOnly()})}
}
func (r *ASomeTime) Expr() expr.Expr { return r.expression }
func (r *ASomeTime) Description() string {
	return "Removes the redundant/conflicting indefinite article `a` before `some` when followed by time expressions."
}
func (r *ASomeTime) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ASomeTime) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span, ok := doc.Span(t[:3])
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("some", span.GetContent(s))}, Message: "Remove the indefinite article `a` before `some`.", Priority: 127}
}
