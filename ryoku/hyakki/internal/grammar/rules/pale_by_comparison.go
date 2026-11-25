package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type PaleByComparison struct{ expression expr.Expr }

func NewPaleByComparison() *PaleByComparison {
	return &PaleByComparison{expression: expr.NewSequenceExprWith(expr.NewInflectionOfBe()).TWs().TAco("pale").TWs().TSet([]string{"by", "in"}).TWs().TAco("comparison")}
}
func (r *PaleByComparison) Expr() expr.Expr { return r.expression }
func (r *PaleByComparison) Description() string {
	return "Corrects variants of the idiom \"pale by/in comparison\" where pale is incorrectly used as an adjective instead of a verb."
}
func (r *PaleByComparison) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *PaleByComparison) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	form := ""
	for _, p := range [][2]string{{"am", "pale"}, {"are", "pale"}, {"be", "pale"}, {"been", "paled"}, {"being", "paling"}, {"is", "pales"}, {"was", "paled"}, {"were", "paled"}} {
		if text.EqStr(t[0].GetCh(s), p[0]) {
			form = p[1]
			break
		}
	}
	if form == "" {
		return nil
	}
	span, ok := doc.Span(t[:3])
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Eggcorn, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr(form, span.GetContent(s))}, Message: "In this idiom, the word `pale` is correctly a verb, not an adjective.", Priority: 127}
}
