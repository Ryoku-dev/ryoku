package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type AfterLater struct{ expression expr.Expr }

func NewAfterLater() *AfterLater {
	return &AfterLater{expression: expr.NewSequenceExprAco("after").TWs().ThenOptional(expr.NewSequenceExprWordSet([]string{"about", "almost", "approximately", "around", "circa", "exactly", "just", "maybe", "nearly", "only", "perhaps", "precisely", "probably", "roughly"}).TWs()).Then(expr.NewDurationExpr()).TWs().TAco("later")}
}
func (r *AfterLater) Expr() expr.Expr { return r.expression }
func (r *AfterLater) Description() string {
	return "Checks for the word `later` following `after [a period of time]`."
}
func (r *AfterLater) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *AfterLater) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Redundancy

	a, _ := doc.Span(toks[2:])
	b, _ := doc.Span(toks[:len(toks)-2])
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCase(src[a.Start:a.End], src[span.Start:span.End]), lint.ReplaceWithMatchCase(src[b.Start:b.End], src[span.Start:span.End])}
	result.Message = "Don't use `later` following `after [a period of time]`"
	return &result
}
