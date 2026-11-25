package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type ThrowAway struct{ expression expr.Expr }

func NewThrowAway() *ThrowAway {
	return &ThrowAway{expression: expr.NewSequenceExprAco("through").TWs().TAco("away")}
}
func (r *ThrowAway) Expr() expr.Expr                  { return r.expression }
func (r *ThrowAway) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ThrowAway) Description() string {
	return "Finds the typo `through away` and suggests `throw away` or `threw away` instead."
}
func (r *ThrowAway) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	tok := toks[0]
	l := lint.NewLint()
	l.Span = tok.Span
	l.LintKind = lint.Typo
	l.Priority = 60
	l.Message = "Use `throw away` or `threw away`, depending on the tense you need."
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("throw", tok.GetCh(src)), lint.ReplaceWithMatchCaseStr("threw", tok.GetCh(src))}
	return &l
}
