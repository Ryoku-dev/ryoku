package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type InFavourOfDoing struct{ expression expr.Expr }

func NewInFavourOfDoing() *InFavourOfDoing {
	return &InFavourOfDoing{expr.NewSequenceExprAco("in").TWs().TSet([]string{"favour", "favor"}).TWs().ThenVerbProgressiveForm()}
}
func (r *InFavourOfDoing) Expr() expr.Expr                  { return r.expression }
func (r *InFavourOfDoing) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*InFavourOfDoing) Description() string {
	return "Corrects missing `of` in `in favor/favour of doing`, etc."
}
func (r *InFavourOfDoing) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) < 3 {
		return nil
	}
	return &lint.Lint{Span: t[2].Span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.InsertAfter([]rune(" of"))}, Message: "The word `of` is missing.", Priority: 127}
}
