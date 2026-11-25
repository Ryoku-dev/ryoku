package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type NotOnlyInversion struct{ expression expr.Expr }

func NewNotOnlyInversion() *NotOnlyInversion {
	return &NotOnlyInversion{expression: expr.NewSequenceExprWordSeq([]string{"not", "only"}).TWs().ThenWordSet([]string{"I", "we", "you", "he", "she", "it", "they"}).TWs().ThenWordSet([]string{"am", "are", "is", "was", "were"})}
}
func (r *NotOnlyInversion) Expr() expr.Expr                  { return r.expression }
func (r *NotOnlyInversion) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *NotOnlyInversion) Description() string {
	return "Corrects `not only it is` to `not only is it`"
}
func (r *NotOnlyInversion) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) < 3 {
		return nil
	}
	pron, be := t[len(t)-3], t[len(t)-1]
	span, ok := doc.Span(t[len(t)-3:])
	if !ok {
		return nil
	}
	inverted := be.GetStr(s) + " " + pron.GetStr(s)
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Grammar
	result.Message = "After `not only` the subject and verb should be inverted."
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(inverted, s[span.Start:span.End])}
	return &result
}
