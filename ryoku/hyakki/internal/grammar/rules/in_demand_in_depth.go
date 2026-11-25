package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type InDemandInDepth struct{ expression expr.Expr }

func NewInDemandInDepth() *InDemandInDepth {
	return &InDemandInDepth{expression: expr.NewSequenceExpr().ThenKindAnyButNot([]expr.KindPredicate{doc.TokenKind.IsVerbLemma, doc.TokenKind.IsPreposition, doc.TokenKind.IsQuantifier, doc.TokenKind.IsDegreeAdverb}, doc.TokenKind.IsMassNoun).TWs().TAco("in").TWs().TSet([]string{"demand", "depth"})}
}
func (r *InDemandInDepth) Expr() expr.Expr                  { return r.expression }
func (r *InDemandInDepth) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *InDemandInDepth) Description() string {
	return "Checks for `in-demand` and `in-depth` used as adjectives but not hyphenated."
}
func (r *InDemandInDepth) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 4 {
		return nil
	}
	l := lint.NewLint()
	l.Span = toks[3].Span
	l.LintKind = lint.Punctuation
	l.Suggestions = []lint.Suggestion{lint.ReplaceWith([]rune("-"))}
	l.Message = "When used as adjectives, `in-demand` and `in-depth` should be hyphenated."
	return &l
}
