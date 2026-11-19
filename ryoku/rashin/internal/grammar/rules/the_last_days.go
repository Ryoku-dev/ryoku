package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type TheLastDays struct{ expression expr.Expr }

func NewTheLastDays() *TheLastDays {
	return &TheLastDays{expression: expr.ButNot(expr.NewSequenceExpr().ThenPreposition().TWs().TAco("the").TWs().TAco("last").TWs().ThenWordSet([]string{"seconds", "minutes", "hours", "days", "weeks", "months", "years", "decades"}), expr.NewSequenceExprAnything().TAny().TAny().TAny().TAny().TAny().TAny().TWs().TAco("of"))}
}
func (r *TheLastDays) Expr() expr.Expr { return r.expression }
func (r *TheLastDays) Description() string {
	return "Corrects `in the last days` to `in the last few days` and related errors."
}
func (r *TheLastDays) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *TheLastDays) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span := toks[4].Span
	suggestions := []lint.Suggestion{lint.InsertAfter([]rune(" few"))}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Usage
	result.Message = "To speak about recent time use `the last few` rather than just `the last`."
	result.Suggestions = suggestions
	return &result
}
