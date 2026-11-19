package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type MoreBetter struct{ expression expr.Expr }

func NewMoreBetter() *MoreBetter {
	return &MoreBetter{expression: expr.NewSequenceExprAnyOf([]expr.Expr{expr.NewSequenceExprAco("more").TWs().ThenComparativeAdjective(), expr.NewSequenceExprAco("most").TWs().ThenSuperlativeAdjective()})}
}
func (r *MoreBetter) Expr() expr.Expr                  { return r.expression }
func (r *MoreBetter) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *MoreBetter) Description() string {
	return "Finds redundant paring of `more` or `most` with adjectives already in the comparative or superlative form."
}
func (r *MoreBetter) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	degree := toks[0].GetStr(src)
	adj := toks[len(toks)-1]
	form := "superlative"
	if strings.EqualFold(degree, "more") {
		form = "comparative"
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Redundancy
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCase(adj.GetCh(src), span.GetContent(src))}
	l.Message = fmt.Sprintf("%s is already in the %s form, the %s is redundant", adj.GetStr(src), form, degree)
	return &l
}
