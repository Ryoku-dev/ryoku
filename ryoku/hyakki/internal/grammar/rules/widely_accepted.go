package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type WidelyAccepted struct{ expression expr.Expr }

func NewWidelyAccepted() *WidelyAccepted {
	return &WidelyAccepted{expression: expr.NewSequenceExpr().TAco("wide").ThenWhitespace().ThenWordSet([]string{"accepted", "acceptable", "used"})}
}
func (r *WidelyAccepted) Expr() expr.Expr { return r.expression }
func (r *WidelyAccepted) Description() string {
	return "Flags `wide accepted`, `wide acceptable`, or `wide used` and recommends switching `wide` to the adverb `widely`."
}
func (r *WidelyAccepted) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *WidelyAccepted) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	return &lint.Lint{Span: t[0].Span, LintKind: lint.Miscellaneous, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("widely", t[0].GetCh(s))}, Message: "Use the adverb `widely` in this context. For example, `widely accepted` or `widely used` is standard usage.", Priority: 31}
}
