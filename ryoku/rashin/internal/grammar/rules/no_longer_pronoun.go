package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type NoLongerPronoun struct{ expression expr.Expr }

func NewNoLongerPronoun() *NoLongerPronoun {
	return &NoLongerPronoun{expression: expr.NewSequenceExprAco("no").TWs().TAco("longer").TWs().ThenSubjectPronoun().ThenOptional(expr.NewSequenceExprWhitespace().TSet([]string{"am", "are", "is"}))}
}
func (r *NoLongerPronoun) Expr() expr.Expr                  { return r.expression }
func (r *NoLongerPronoun) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*NoLongerPronoun) Description() string {
	return "Detects incorrect word order where `no longer` incorrectly precedes a subject pronoun."
}
func (r *NoLongerPronoun) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return r.MatchToLintWithContext(t, s, nil)
}
func (r *NoLongerPronoun) MatchToLintWithContext(t []doc.Token, s []rune, ctx *lint.MatchContext) *lint.Lint {
	if lint.FollowedByWord(ctx, func(tok *doc.Token) bool { return expr.NewRelativePronoun().MatchesToken(*tok, s) }) {
		return nil
	}
	span, ok := doc.Span(t)
	if !ok {
		return nil
	}
	var correction []rune
	for _, part := range [][]doc.Token{t[4:], t[3:4], t[:3]} {
		for _, tok := range part {
			correction = append(correction, tok.GetCh(s)...)
		}
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.WordOrder
	l.Suggestions = []lint.Suggestion{lint.ReplaceWith(correction)}
	l.Message = "Consider moving the pronoun to before `no longer`."
	return &l
}
