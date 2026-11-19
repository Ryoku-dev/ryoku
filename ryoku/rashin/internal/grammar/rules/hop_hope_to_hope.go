package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type ToHope struct{ expression expr.Expr }

func NewToHope() *ToHope {
	return &ToHope{expression: expr.NewSequenceExpr().ThenNominal().TWs().ThenWordSet([]string{"hop", "hopped"}).TWs().ThenNominal()}
}
func (r *ToHope) Expr() expr.Expr { return r.expression }
func (r *ToHope) Description() string {
	return "Detects incorrect use of 'hop' when the correct verb 'hope' should be used in a sentence."
}
func (r *ToHope) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ToHope) MatchToLint(tokens []doc.Token, source []rune) *lint.Lint {
	if len(tokens) <= 2 {
		return nil
	}
	target := tokens[2]
	result := lint.NewLint()
	result.Span = target.Span
	result.LintKind = lint.WordChoice
	result.Priority = 127
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("hope", target.GetCh(source))}
	result.Message = "Did you mean to use 'hope' instead of 'hop' in this context?"
	return &result
}
