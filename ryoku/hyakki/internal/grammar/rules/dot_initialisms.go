package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type DotInitialisms struct{ expression expr.Expr }

func NewDotInitialisms() *DotInitialisms {
	e := expr.NewWordExprGroup()
	for _, w := range []string{"ie", "eg"} {
		e.Add(w, expr.NewSequenceExpr().ThenExactWord(w).ThenPunctuation())
	}
	return &DotInitialisms{expression: e}
}
func (r *DotInitialisms) Expr() expr.Expr { return r.expression }
func (r *DotInitialisms) Description() string {
	return "Ensures common initialisms (like \"i.e.\") are properly dot-separated."
}
func (r *DotInitialisms) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *DotInitialisms) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	var correction string
	switch t[0].GetStr(s) {
	case "ie":
		correction = "i.e."
	case "eg":
		correction = "e.g."
	default:
		return nil
	}
	span, ok := doc.Span(t)
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Formatting, Suggestions: []lint.Suggestion{lint.ReplaceWith([]rune(correction))}, Message: "Initialisms should have dot-separated letters.", Priority: 63}
}
