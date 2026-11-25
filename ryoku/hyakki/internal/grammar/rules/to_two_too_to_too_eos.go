package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type ToTooEos struct{ expression expr.Expr }

func NewToTooEos() *ToTooEos {
	return &ToTooEos{expression: expr.NewSequenceExpr().ThenComma().TWs().TAco("to").ThenSentenceTerminator()}
}
func (r *ToTooEos) Description() string {
	return "Identifies incorrect usage of the term `to` at the end of a sentence."
}
func (r *ToTooEos) Lint(d *doc.Document) []lint.Lint {
	var result []lint.Lint
	for _, span := range expr.IterMatchesInDoc(r.expression, d) {
		target := d.Tokens()[span.Start+2]
		result = append(result, lint.Lint{Span: target.Span, LintKind: lint.WordChoice, Priority: 63, Message: "Use `too` when expressing similarity.", Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("too", target.GetCh(d.GetSource()))}})
	}
	return result
}
