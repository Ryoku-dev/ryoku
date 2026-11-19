package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type MultipleFrequencyAdverbs struct{ expression expr.Expr }

func NewMultipleFrequencyAdverbs() *MultipleFrequencyAdverbs {
	adverb := func(t doc.Token, s []rune) bool { return t.Kind.IsFrequencyAdverb() && !text.EqStr(t.GetCh(s), "only") }
	return &MultipleFrequencyAdverbs{expression: expr.NewSequenceExpr().Then(adverb).ThenOptionalComma().TWs().Then(adverb)}
}
func (r *MultipleFrequencyAdverbs) Expr() expr.Expr                  { return r.expression }
func (r *MultipleFrequencyAdverbs) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *MultipleFrequencyAdverbs) Description() string {
	return "Looks for adjacent adverbs of frequency, which will be either redundant or contradictory."
}
func (r *MultipleFrequencyAdverbs) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	a := toks[0].GetCh(src)
	b := toks[len(toks)-1].GetCh(src)
	if text.EqCh(a, b) {
		return nil
	}
	span, _ := doc.Span(toks)
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Usage
	l.Suggestions = []lint.Suggestion{}
	l.Message = fmt.Sprintf("The adverbs of frequency ‘%s’ and ‘%s’ are either redundant or contradictory", string(a), string(b))
	return &l
}
func (*MultipleFrequencyAdverbs) Unit() lint.DocumentIterator { return lint.Sentence{} }
