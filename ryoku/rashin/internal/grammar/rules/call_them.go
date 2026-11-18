package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type CallThem struct {
	expression expr.Expr
	removals   *expr.ExprMap[text.Span]
}

func NewCallThem() *CallThem {
	m := expr.NewExprMap[text.Span]()
	post := expr.NewSequenceExpr().TWs().TSet([]string{"if", "it"})
	m.Insert(expr.NewSequenceExprWith(expr.NewDerivedFromStr("call")).TWs().ThenPronoun().TWs().TAco("as").ThenUnless(post), text.Span{Start: 3, End: 5})
	m.Insert(expr.NewSequenceExprWith(expr.NewDerivedFromStr("call")).TWs().TAco("as").TWs().ThenPronoun().ThenUnless(post), text.Span{Start: 1, End: 3})
	e := m
	return &CallThem{expression: e, removals: m}
}
func (r *CallThem) Expr() expr.Expr                  { return r.expression }
func (r *CallThem) Description() string              { return "Addresses the non-idiomatic phrases `call them as`." }
func (r *CallThem) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *CallThem) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	rem, ok := r.removals.Lookup(0, t, s)
	if !ok {
		return nil
	}
	span, ok := doc.Span(t[rem.Start:rem.End])
	if !ok {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Redundancy
	l.Suggestions = []lint.Suggestion{lint.Remove()}
	l.Message = "`as` is redundant in this context."
	return &l
}
