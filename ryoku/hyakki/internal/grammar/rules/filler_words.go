package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type FillerWords struct{ expression expr.Expr }

func NewFillerWords() *FillerWords {
	return &FillerWords{expression: expr.NewSequenceExprAnyOf([]expr.Expr{expr.NewSequenceExprWordSet([]string{"uh", "um"}).ThenWhitespace(), expr.NewSequenceExprWhitespace().ThenWordSet([]string{"uh", "um"})})}
}
func (r *FillerWords) Expr() expr.Expr                  { return r.expression }
func (r *FillerWords) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*FillerWords) Description() string                { return "Removes filler words." }
func (r *FillerWords) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span, ok := doc.Span(t)
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Miscellaneous, Suggestions: []lint.Suggestion{lint.Remove()}, Message: "Remove this unnecessary filler word.", Priority: 31}
}
