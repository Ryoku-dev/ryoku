package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type CoolCalmCollected struct{ expression expr.Expr }

func NewCoolCalmCollected() *CoolCalmCollected {
	return &CoolCalmCollected{expression: expr.NewSequenceExprAco("cool").ThenOptionalComma().TWs().TAco("calm").ThenOptionalComma().TWs().TAco("and").ThenOptionalComma().TWs().TAco("collective")}
}
func (r *CoolCalmCollected) Expr() expr.Expr { return r.expression }
func (r *CoolCalmCollected) Description() string {
	return "Corrects the malapropism `cool, calm, and collective` to use the correct `collected`."
}
func (r *CoolCalmCollected) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *CoolCalmCollected) Unit() lint.DocumentIterator      { return lint.Sentence{} }
func (r *CoolCalmCollected) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	span := toks[len(toks)-1].Span
	suggestions := []lint.Suggestion{lint.ReplaceWithMatchCaseStr("collected", src[span.Start:span.End])}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Malapropism
	result.Message = "`A `collective` is a kind of group. This idiom uses the word `collected`.`"
	result.Suggestions = suggestions
	return &result
}
