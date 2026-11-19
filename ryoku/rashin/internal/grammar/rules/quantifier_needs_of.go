package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

// QuantifierNeedsOf flags phrases like `a couple months` that need `of`.
type QuantifierNeedsOf struct{ expression expr.Expr }

func NewQuantifierNeedsOf() *QuantifierNeedsOf {
	return &QuantifierNeedsOf{expression: expr.NewSequenceExpr().ThenIndefiniteArticle().TWs().ThenWordSet([]string{"couple", "lot"}).TWs().ThenPluralNominal()}
}
func (r *QuantifierNeedsOf) Expr() expr.Expr                  { return r.expression }
func (r *QuantifierNeedsOf) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *QuantifierNeedsOf) Description() string {
	return "Detects missing `of` after the quantifier “a couple” when it precedes a plural noun"
}
func (r *QuantifierNeedsOf) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 3 {
		return nil
	}
	return &lint.Lint{Span: toks[2].Span, LintKind: lint.Miscellaneous, Suggestions: []lint.Suggestion{lint.InsertAfter([]rune(" of"))}, Message: "Add `of` in this quantity phrase.", Priority: 32}
}
