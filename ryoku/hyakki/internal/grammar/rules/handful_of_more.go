package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type HandfulOfMore struct{ expression expr.Expr }

func NewHandfulOfMore() *HandfulOfMore {
	e := expr.NewSequenceExprAco("handful").TWs().TAco("of").TWs().TAco("more").TWs().ThenKindIsButIsNot(doc.TokenKind.IsNoun, doc.TokenKind.IsAdjective)
	return &HandfulOfMore{expression: e}
}
func (r *HandfulOfMore) Expr() expr.Expr { return r.expression }
func (r *HandfulOfMore) Description() string {
	return "Removes the nonstandard `of` from `handful of more`."
}
func (r *HandfulOfMore) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *HandfulOfMore) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span, _ := doc.Span(t[1:3])
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Nonstandard
	l.Suggestions = []lint.Suggestion{lint.Remove()}
	l.Message = "Using `of` in this construction is not standard."
	return &l
}
