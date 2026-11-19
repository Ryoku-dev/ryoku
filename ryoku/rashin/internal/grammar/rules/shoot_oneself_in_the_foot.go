package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ShootOneselfInTheFoot struct{ expression expr.Expr }

func NewShootOneselfInTheFoot() *ShootOneselfInTheFoot {
	return &ShootOneselfInTheFoot{expression: expr.NewSequenceExprWordSet([]string{"shoot", "shooting", "shoots", "shot", "shooted"}).TWs().Then(expr.NewReflexivePronoun()).TWs().ThenPreposition().TWs().ThenDeterminer().TWs().Then(expr.NewWordSet([]string{"foot", "feet", "leg", "legs"}))}
}
func (r *ShootOneselfInTheFoot) Expr() expr.Expr { return r.expression }
func (r *ShootOneselfInTheFoot) Description() string {
	return "Corrects nonstandard variants of 'shoot oneself in the foot'."
}
func (r *ShootOneselfInTheFoot) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ShootOneselfInTheFoot) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) < 9 {
		return nil
	}
	plural := text.EndsWithIgnoreASCIICaseStr(t[2].GetCh(s), "elves")
	if text.EqStr(t[4].GetCh(s), "in") && text.EqStr(t[6].GetCh(s), "the") && (text.EqStr(t[8].GetCh(s), "foot") || (plural && t[8].Kind.IsPluralNoun())) {
		return nil
	}
	span := text.Span{Start: t[4].Span.Start, End: t[8].Span.End}
	result := lint.NewLint()
	result.Span = span
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("in the foot", span.GetContent(s))}
	if plural {
		result.Suggestions = append(result.Suggestions, lint.ReplaceWithMatchCaseStr("in the feet", span.GetContent(s)))
	}
	result.Message = "The standard idiom is 'shoot oneself in the foot'."
	result.Priority = 50
	return &result
}
