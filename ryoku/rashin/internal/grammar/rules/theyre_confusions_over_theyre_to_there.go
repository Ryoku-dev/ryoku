package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type OverTheyreToThere struct{ expression expr.Expr }

func NewOverTheyreToThere() *OverTheyreToThere {
	return &OverTheyreToThere{expression: expr.NewSequenceExprAco("over").TWs().Then(theyreConfusionsTokenIsTheyre)}
}
func (r *OverTheyreToThere) Expr() expr.Expr { return r.expression }
func (r *OverTheyreToThere) Description() string {
	return "Corrects locative `over they're` to `over there`."
}
func (r *OverTheyreToThere) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *OverTheyreToThere) MatchToLint(tokens []doc.Token, source []rune) *lint.Lint {
	if len(tokens) == 0 {
		return nil
	}
	target := tokens[len(tokens)-1]
	result := lint.NewLint()
	result.Span = target.Span
	result.LintKind = lint.Grammar
	result.Priority = 31
	result.Message = "Did you mean `there`?"
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("there", target.GetCh(source))}
	return &result
}
