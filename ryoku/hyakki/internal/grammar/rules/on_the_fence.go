package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type OnTheFence struct{ expression expr.Expr }

func NewOnTheFence() *OnTheFence {
	return &OnTheFence{expression: expr.NewSequenceExprAnyOf([]expr.Expr{expr.AsExpr(expr.NewWordSet([]string{"i'm", "we're", "you're", "he's", "she's", "they're"})), expr.NewSequenceExprAco("i").TWs().TSet([]string{"am", "was"}), expr.NewSequenceExprWordSet([]string{"we", "you", "they"}).TWs().TSet([]string{"are", "were"}), expr.NewSequenceExprWordSet([]string{"he", "she", "anybody", "anyone"}).TWs().TSet([]string{"is", "was"})}).ThenOptional(expr.NewSequenceExprWhitespace().TSet([]string{"also", "still"})).TWs().ThenWordSeq([]string{"on", "a", "fence"}).ThenAnyOf([]expr.Expr{expr.NewAnchorEnd(), expr.NewSequenceExprWhitespace().ThenAnyOf([]expr.Expr{expr.NewSequenceExpr().ThenPreposition(), expr.NewAnchorEnd()})})}
}
func (r *OnTheFence) Expr() expr.Expr                  { return r.expression }
func (r *OnTheFence) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *OnTheFence) Description() string {
	return "Corrects `on a fence` to `on the fence` for the idiom meaning 'undecided'."
}
func (r *OnTheFence) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	i, ok := lint.FindTheOnlyTokenIndexMatching(toks, src, func(t *doc.Token, s []rune) bool { return text.EqStr(t.GetCh(s), "fence") })
	if !ok || i < 2 {
		return nil
	}
	span := toks[i-2].Span
	return &lint.Lint{Span: span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("the", span.GetContent(src))}, Message: "If this is the idiom meaning `undecided`, it should be `on the fence`.", Priority: 127}
}
