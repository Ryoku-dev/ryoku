package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type Addicting struct{ expression expr.Expr }

func NewAddicting() *Addicting {
	return &Addicting{expression: expr.NewSequenceExprAco("addicting").ThenLongestOf([]expr.Expr{expr.NewAnchorEnd(), expr.NewSequenceExpr().ThenSemicolon(), expr.NewSequenceExprWhitespace().Then(expr.NewAll([]expr.Expr{expr.NewSequenceExprAnyWord(), expr.NewSequenceExprUnless(expr.NewFirstMatchOf([]expr.Expr{expr.AsExpr(expr.NewReflexivePronounWithCommonErrors()), expr.NewSequenceExpr().ThenObjectPronoun()}))}))})}
}
func (r *Addicting) Expr() expr.Expr                  { return r.expression }
func (r *Addicting) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *Addicting) Description() string {
	return "Replaces `addicting` with `addictive` when used as an adjective."
}
func (r *Addicting) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	tok := toks[0]
	l := lint.NewLint()
	l.Span = tok.Span
	l.LintKind = lint.Style
	l.Message = "When used as an adjective, `addictive` is the traditional and more f form."
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("addictive", tok.GetCh(src))}
	return &l
}
