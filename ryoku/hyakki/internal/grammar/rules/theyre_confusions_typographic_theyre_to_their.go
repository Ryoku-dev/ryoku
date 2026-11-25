package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type TypographicTheyreToTheir struct{ expression expr.Expr }

func NewTypographicTheyreToTheir() *TypographicTheyreToTheir {
	return &TypographicTheyreToTheir{expression: expr.NewSequenceExprWith(theyreConfusionsTokenIsTypographicTheyre).TWs().Then(theyreConfusionsTokenIsLikelyTheirPossession)}
}
func (r *TypographicTheyreToTheir) Expr() expr.Expr { return r.expression }
func (r *TypographicTheyreToTheir) Description() string {
	return "Corrects smart-apostrophe `they’re` to `their` in possessive noun contexts."
}
func (r *TypographicTheyreToTheir) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *TypographicTheyreToTheir) MatchToLint(tokens []doc.Token, source []rune) *lint.Lint {
	if len(tokens) <= 0 {
		return nil
	}
	target := tokens[0]
	result := lint.NewLint()
	result.Span = target.Span
	result.LintKind = lint.Grammar
	result.Priority = 31
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("their", target.GetCh(source))}
	result.Message = "Did you mean `their`?"
	return &result
}
