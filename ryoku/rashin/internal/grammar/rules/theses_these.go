package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/pos"
)

type ThesesThese struct{ expression expr.Expr }

func NewThesesThese() *ThesesThese {
	return &ThesesThese{expression: expr.NewSequenceExprAco("theses").TWs().Then(expr.NewUPOSSet([]pos.UPOS{pos.NOUN, pos.PROPN}))}
}
func (r *ThesesThese) Expr() expr.Expr                  { return r.expression }
func (r *ThesesThese) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ThesesThese) Description() string {
	return "Corrects the common misspelling of `these` as `theses`."
}
func (r *ThesesThese) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	span := toks[0].Span
	return &lint.Lint{Span: span, LintKind: lint.Spelling, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("these", span.GetContent(src))}, Message: "Did you mean `these`?", Priority: 1}
}
