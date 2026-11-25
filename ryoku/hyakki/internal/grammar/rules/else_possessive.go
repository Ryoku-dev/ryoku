package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type ElsePossessive struct{ expression expr.Expr }

func NewElsePossessive() *ElsePossessive {
	return &ElsePossessive{expression: expr.NewSequenceExprWith(expr.Or(expr.NewWordSet([]string{"somebody", "someone", "anybody", "anyone", "everybody", "everyone", "nobody"}), expr.NewSequenceExprWordSeq([]string{"no", "one"}))).ThenWhitespace().TAco("elses")}
}
func (r *ElsePossessive) Expr() expr.Expr                  { return r.expression }
func (r *ElsePossessive) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ElsePossessive) Description() string {
	return "Detects missing apostrophes in phrases like `someone elses book` and suggests the correct possessive form `else’s`."
}
func (r *ElsePossessive) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	return &lint.Lint{Span: t[len(t)-1].Span, LintKind: lint.Miscellaneous, Suggestions: []lint.Suggestion{lint.ReplaceWith([]rune("else's"))}, Message: "Add the missing possessive apostrophe: use `else’s`.", Priority: 60}
}
