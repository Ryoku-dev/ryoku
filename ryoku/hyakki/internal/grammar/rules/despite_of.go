package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type DespiteOf struct{ expression expr.Expr }

func NewDespiteOf() *DespiteOf {
	return &DespiteOf{expression: expr.NewSequenceExprAco("despite").ThenWhitespace().ThenExactWord("of")}
}
func (r *DespiteOf) Expr() expr.Expr { return r.expression }
func (r *DespiteOf) Description() string {
	return "Corrects the misuse of `despite of` and suggests the proper alternatives `despite` or `in spite of`."
}
func (r *DespiteOf) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *DespiteOf) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span, ok := doc.Span(t)
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("despite", span.GetContent(s)), lint.ReplaceWithMatchCaseStr("in spite of", span.GetContent(s))}, Message: "The phrase “despite of” is incorrect. Please use either “despite” or “in spite of” instead.", Priority: 126}
}
