package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type LeftRightHand struct{ expression expr.Expr }

func NewLeftRightHand() *LeftRightHand {
	return &LeftRightHand{expression: expr.NewSequenceExprWordSet([]string{"left", "right"}).ThenWhitespace().TAco("hand").ThenWhitespace().ThenNoun()}
}
func (r *LeftRightHand) Expr() expr.Expr { return r.expression }
func (r *LeftRightHand) Description() string {
	return "Ensures `left hand` and `right hand` are hyphenated when used as adjectives before a noun, such as in `left-hand side` or `right-hand corner`."
}
func (r *LeftRightHand) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *LeftRightHand) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return &lint.Lint{Span: t[1].Span, LintKind: lint.Miscellaneous, Suggestions: []lint.Suggestion{lint.ReplaceWith([]rune("-"))}, Message: "Use a hyphen in `left-hand` or `right-hand` when modifying a noun.", Priority: 31}
}
