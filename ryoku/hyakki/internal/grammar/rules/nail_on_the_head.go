package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type NailOnTheHead struct{ expression expr.Expr }

func NewNailOnTheHead() *NailOnTheHead {
	return &NailOnTheHead{expression: expr.NewSequenceExprWordSeq([]string{"nail", "on", "the"}).TWs().TSet([]string{"hat", "had", "hit", "hid"})}
}
func (r *NailOnTheHead) Expr() expr.Expr                  { return r.expression }
func (r *NailOnTheHead) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *NailOnTheHead) Description() string {
	return "Replaces hat/had/hit/hid in the idiom `nail on the head` with `head`."
}
func (r *NailOnTheHead) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	span := toks[len(toks)-1].Span
	return &lint.Lint{Span: span, LintKind: lint.Miscellaneous, Suggestions: []lint.Suggestion{lint.ReplaceWith([]rune("head"))}, Message: "Did you mean `head`?", Priority: 45}
}
