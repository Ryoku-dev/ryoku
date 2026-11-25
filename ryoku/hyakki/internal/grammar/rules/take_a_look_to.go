package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type TakeALookTo struct{ expression expr.Expr }

func NewTakeALookTo() *TakeALookTo {
	e := expr.NewSequenceExprAnyOf([]expr.Expr{expr.AsExpr(expr.NewWordSet(takeALookToTakeForms)), expr.AsExpr(expr.NewWordSet(takeALookToHaveForms))}).TWs().TAco("a").TWs().TAco("look").TWs().TAco("to")
	return &TakeALookTo{expression: e}
}
func (r *TakeALookTo) Expr() expr.Expr { return r.expression }
func (r *TakeALookTo) Description() string {
	return "Corrects `take a look to`/`have a look to` to correctly use `at`."
}
func (r *TakeALookTo) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *TakeALookTo) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return r.MatchToLintWithContext(t, s, nil)
}

var takeALookToTakeForms = []string{"take", "took", "taken", "takes", "taking"}
var takeALookToHaveForms = []string{"have", "had", "has", "having"}

func (r *TakeALookTo) MatchToLintWithContext(t []doc.Token, s []rune, c *lint.MatchContext) *lint.Lint {
	if lint.FollowedByWord(c, func(n *doc.Token) bool {
		return n.Kind.IsVerbLemma() || (text.EqStr(n.GetCh(s), "it") && len(t) > 0 && text.EqAnyIgnoreASCIICaseStr(t[0].GetCh(s), takeALookToHaveForms))
	}) {
		return nil
	}
	if len(t) == 0 {
		return nil
	}
	tok := t[len(t)-1]
	l := lint.NewLint()
	l.Span = tok.Span
	l.LintKind = lint.Usage
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("at", tok.GetCh(s))}
	l.Message = "This phrase uses `to` rather than `at`"
	return &l
}
