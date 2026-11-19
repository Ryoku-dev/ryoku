package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type SeemsLikeAdjective struct{ expression expr.Expr }

func NewSeemsLikeAdjective() *SeemsLikeAdjective {
	return &SeemsLikeAdjective{expression: expr.NewSequenceExprWordSet([]string{"appear", "appears", "feel", "feels", "look", "looks", "seem", "seems", "smell", "smells", "sound", "sounds", "taste", "tastes"}).TWs().TAco("like").TWs().ThenKindIsButIsNot(doc.TokenKind.IsAdjective, doc.TokenKind.IsNoun)}
}
func (r *SeemsLikeAdjective) Expr() expr.Expr { return r.expression }
func (r *SeemsLikeAdjective) Description() string {
	return "Detects verbs of perception redundantly followed by `like` when used with an adjective."
}
func (r *SeemsLikeAdjective) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *SeemsLikeAdjective) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks[1:3])
	if !ok {
		return nil
	}
	suggestions := []lint.Suggestion{lint.Remove()}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Redundancy
	result.Message = "Verbs of perception should not be followed by 'like'."
	result.Suggestions = suggestions
	return &result
}
