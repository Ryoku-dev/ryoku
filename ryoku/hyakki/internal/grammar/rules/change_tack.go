package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type ChangeTack struct{ expression expr.Expr }

func NewChangeTack() *ChangeTack {
	verbs := []string{"change", "changes", "changing", "changed"}
	e := expr.NewFirstMatchOf([]expr.Expr{expr.NewSequenceExprLongestOf([]expr.Expr{expr.NewSequenceExprWordSet(verbs).ThenOptional(expr.NewSequenceExpr().TWs().ThenAnyOf([]expr.Expr{expr.NewSequenceExpr().ThenPossessiveDeterminer(), expr.NewWord("it's")})), expr.NewSequenceExprWordSet(verbs[:3]).TWs().TAco("of")}).TWs().ThenWordSet([]string{"tact", "tacks", "tacts"}), expr.NewSequenceExprWordSeq([]string{"different", "tact"})})
	return &ChangeTack{expression: e}
}
func (r *ChangeTack) Expr() expr.Expr                  { return r.expression }
func (r *ChangeTack) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ChangeTack) Description() string {
	return "Locates errors in the idioms `to change tack` and `change of tack` to convey the correct meaning of altering one's course or strategy."
}
func (r *ChangeTack) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	tok := toks[len(toks)-1]
	return &lint.Lint{Span: tok.Span, LintKind: lint.Eggcorn, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("tack", tok.GetCh(src))}, Message: "A change in direction or approach is a change of `tack`. Not `tact` (or `tacks` or `tacts`).", Priority: 32}
}
