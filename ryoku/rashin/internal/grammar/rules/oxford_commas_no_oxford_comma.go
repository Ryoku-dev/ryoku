package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type NoOxfordComma struct{ expression expr.Expr }

func NewNoOxfordComma() *NoOxfordComma {
	return &NoOxfordComma{expression: expr.NewSequenceExpr().Then(expr.NewNominalPhrase()).ThenComma().TWs().Then(expr.NewNominalPhrase()).ThenComma().TWs().TSet([]string{"and", "or", "nor"})}
}
func (r *NoOxfordComma) Description() string {
	return "The Oxford comma is one of the more controversial rules in common use today. Enabling this lint checks that there is no comma before `and`, `or` or `nor` when listing out more than two ideas."
}
func (r *NoOxfordComma) Lint(d *doc.Document) []lint.Lint {
	var out []lint.Lint
	for _, sentence := range d.IterSentences() {
		for _, match := range expr.IterMatches(r.expression, sentence, d.GetSource()) {
			toks := sentence[match.Start:match.End]
			i, ok := doc.LastCommaIndex(toks)
			if !ok {
				continue
			}
			out = append(out, lint.Lint{Span: toks[i].Span, LintKind: lint.Style, Suggestions: []lint.Suggestion{lint.Remove()}, Message: "Remove the Oxford comma here.", Priority: 31})
		}
	}
	return out
}
