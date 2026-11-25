package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/pos"
)

type WayTooAdjective struct{ expression expr.Expr }

func NewWayTooAdjective() *WayTooAdjective {
	return &WayTooAdjective{expression: expr.NewAll([]expr.Expr{expr.NewSequenceExpr().TAco("way").TWs().TAco("to").TWs().Then(expr.Or(expr.NewUPOSSet([]pos.UPOS{pos.ADJ}), expr.NewWordSet([]string{"much"}))), expr.NewSequenceExprUnless(expr.NewSequenceExprAnything().TAny().TAny().TAny().ThenWordSet([]string{"surface", "return", "aqua"}))})}
}
func (r *WayTooAdjective) Expr() expr.Expr { return r.expression }
func (r *WayTooAdjective) Description() string {
	return "Replaces the preposition `to` with the adverb `too` after `way` when followed by an adjective (e.g. `way too fast`)"
}
func (r *WayTooAdjective) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *WayTooAdjective) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) < 3 {
		return nil
	}
	return &lint.Lint{Span: t[2].Span, LintKind: lint.Miscellaneous, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("too", t[2].GetCh(s))}, Message: "Did you mean “too”?", Priority: 25}
}
