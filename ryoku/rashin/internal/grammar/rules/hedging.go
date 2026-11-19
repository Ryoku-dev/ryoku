package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

// Hedging detects hedging language.
type Hedging struct{ expression expr.Expr }

func NewHedging() *Hedging {
	return &Hedging{expression: expr.NewFirstMatchOf([]expr.Expr{expr.NewFixedPhraseFromPhrase("I would argue that"), expr.NewFixedPhraseFromPhrase(", so to speak"), expr.NewFixedPhraseFromPhrase("to a certain degree")})}
}
func (r *Hedging) Expr() expr.Expr                  { return r.expression }
func (r *Hedging) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *Hedging) Description() string {
	return "Flags hedging language (e.g. `I would argue that`, `..., so to speak`, `to a certain degree`)."
}
func (r *Hedging) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Miscellaneous, Suggestions: nil, Message: "You're hedging.", Priority: 31}
}
