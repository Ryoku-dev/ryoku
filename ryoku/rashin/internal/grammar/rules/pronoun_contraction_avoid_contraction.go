package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type AvoidContraction struct{ expression expr.Expr }

func NewAvoidContraction() *AvoidContraction {
	return &AvoidContraction{expression: expr.NewSequenceExprAco("you're").TWs().ThenKindIsButIsNot(doc.TokenKind.IsNominal, doc.TokenKind.IsLikelyHomograph)}
}
func (r *AvoidContraction) Expr() expr.Expr { return r.expression }
func (r *AvoidContraction) Description() string {
	return "This rule looks for situations where a contraction was used where it shouldn't have been."
}
func (r *AvoidContraction) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *AvoidContraction) MatchToLint(tokens []doc.Token, source []rune) *lint.Lint {
	if len(tokens) <= 0 {
		return nil
	}
	target := tokens[0]
	result := lint.NewLint()
	result.Span = target.Span
	result.LintKind = lint.WordChoice
	result.Priority = 63
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("your", target.GetCh(source))}
	result.Message = "It appears you intended to use the possessive version of this word"
	return &result
}
