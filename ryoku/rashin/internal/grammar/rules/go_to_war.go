package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type GoToWar struct{ expression expr.Expr }

func NewGoToWar() *GoToWar {
	return &GoToWar{expression: expr.NewSequenceExprWordSet([]string{"go", "goes", "going", "gone", "went"}).TWs().ThenPreposition().TWs().ThenWordSet([]string{"war"})}
}
func (r *GoToWar) Expr() expr.Expr                  { return r.expression }
func (r *GoToWar) Description() string              { return "Replaces `go at war` with `go to war`." }
func (r *GoToWar) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *GoToWar) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span := toks[2].Span
	if text.EqCh(src[span.Start:span.End], []rune("to")) {
		return nil
	}
	suggestions := []lint.Suggestion{lint.ReplaceWithMatchCaseStr("to", src[span.Start:span.End])}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Usage
	result.Message = "Use `to` instead of `at`."
	result.Suggestions = suggestions
	return &result
}
