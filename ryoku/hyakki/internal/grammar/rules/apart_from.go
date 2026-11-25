package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type ApartFrom struct{ expression expr.Expr }

func NewApartFrom() *ApartFrom {
	return &ApartFrom{expression: expr.NewSequenceExprWordSeq([]string{"apart", "form"})}
}
func (r *ApartFrom) Expr() expr.Expr { return r.expression }
func (r *ApartFrom) Description() string {
	return "Flags the misspelling `apart form` and suggests `apart from`."
}
func (r *ApartFrom) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ApartFrom) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	span := t[len(t)-1].Span
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.WordChoice
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("from", span.GetContent(s))}
	result.Message = "Use `from` to spell `apart from`."
	result.Priority = 50
	return &result
}
