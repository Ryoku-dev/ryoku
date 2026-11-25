package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type FascinatedBy struct{ expression expr.Expr }

func NewFascinatedBy() *FascinatedBy {
	return &FascinatedBy{expression: expr.NewSequenceExprAco("fascinated").TWs().ThenPreposition()}
}
func (r *FascinatedBy) Expr() expr.Expr                  { return r.expression }
func (r *FascinatedBy) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *FascinatedBy) Description() string {
	return "Ensures the correct prepositions are used with `fascinated` (e.g., `fascinated by` or `fascinated with`)."
}
func (r *FascinatedBy) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	span := toks[len(toks)-1].Span
	chars := span.GetContent(src)
	if text.EqAnyIgnoreASCIICaseStr(chars, []string{"by", "with"}) {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Usage
	l.Suggestions = lint.ToReplaceSuggestions([]string{"by", "with"}, chars)
	l.Message = "The correct prepositions to use with `fascinated` are `by` or `with`."
	return &l
}
