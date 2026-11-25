package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type BackInTheDay struct{ expression expr.Expr }

func NewBackInTheDay() *BackInTheDay {
	return &BackInTheDay{expression: expr.OrLongest(expr.NewSequenceExprWith(expr.NewFixedPhraseFromPhrase("back in the days")).TWs().Then(expr.NewWordSet([]string{"before", "of", "when"})), expr.NewFixedPhraseFromPhrase("back in the days"))}
}
func (r *BackInTheDay) Expr() expr.Expr                  { return r.expression }
func (r *BackInTheDay) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *BackInTheDay) Description() string {
	return "This linter flags instances of the nonstandard phrase `back in the days`. The correct, more accepted form is `back in the day`"
}
func (r *BackInTheDay) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) >= 9 {
		return nil
	}
	span, ok := doc.Span(t)
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("back in the day", s[span.Start:span.End])}, Message: "Use the more idiomatic version of this phrase.", Priority: 127}
}
