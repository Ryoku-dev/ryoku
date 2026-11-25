package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type Theres struct{ expression expr.Expr }

func NewTheres() *Theres {
	return &Theres{expression: expr.NewSequenceExprAco("their's").TWs().ThenKindAnyOrWords([]expr.KindPredicate{doc.TokenKind.IsDeterminer, doc.TokenKind.IsQuantifier}, []string{"no", "enough"})}
}
func (r *Theres) Expr() expr.Expr { return r.expression }
func (r *Theres) Description() string {
	return "Replaces the mistaken possessive `their's` before a determiner with the contraction `there's`."
}
func (r *Theres) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *Theres) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	span := t[0].Span
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.WordChoice
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("there's", span.GetContent(s))}
	result.Message = "Use `there's`—the contraction of “there is”—for this construction."
	result.Priority = 31
	return &result
}
