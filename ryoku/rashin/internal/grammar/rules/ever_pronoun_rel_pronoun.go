package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type EverPronounRelPronoun struct{ expression expr.Expr }

func NewEverPronounRelPronoun() *EverPronounRelPronoun {
	return &EverPronounRelPronoun{expression: expr.NewSequenceExprWordSet([]string{"whatever", "whenever", "wherever", "whichever", "whoever"}).TWs().ThenAnyOf([]expr.Expr{expr.NewSequenceExprAco("that").TWs().ThenAnyOf([]expr.Expr{expr.NewSequenceExpr().ThenSubjectPronoun(), expr.AsExpr(expr.NewWordSet([]string{"I've", "we've", "you've", "he's", "she's", "it's", "they've"}))}), expr.AsExpr(expr.NewWordSet([]string{"which", "who", "whom"}))})}
}
func (r *EverPronounRelPronoun) Expr() expr.Expr { return r.expression }
func (r *EverPronounRelPronoun) Description() string {
	return "Removes unnecessary redundant relative pronoun after `whatever`, `whoever`, etc."
}
func (r *EverPronounRelPronoun) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *EverPronounRelPronoun) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Redundancy

	result.Span, _ = doc.Span(toks[1:3])
	result.Suggestions = []lint.Suggestion{lint.Remove()}
	result.Message = "The relative pronoun is not needed here."
	return &result
}
