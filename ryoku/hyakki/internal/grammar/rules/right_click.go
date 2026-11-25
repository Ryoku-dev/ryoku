package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type RightClick struct {
	expression expr.Expr
	mapping    *expr.ExprMap[int]
}

func NewRightClick() *RightClick {
	e := expr.NewExprMap[int]()
	e.Insert(expr.NewSequenceExprWordSet([]string{"right", "left", "middle"}).TWs().Then(expr.NewDerivedFromStr("click")), 0)
	return &RightClick{expression: e, mapping: e}
}
func (r *RightClick) Expr() expr.Expr                  { return r.expression }
func (r *RightClick) Description() string              { return "Hyphenates right-click style mouse commands." }
func (r *RightClick) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *RightClick) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	start, ok := r.mapping.Lookup(0, t, s)
	if !ok || len(t) == 0 {
		return nil
	}
	end := len(t) - 1
	if start > end {
		return nil
	}
	span, ok := doc.Span(t[start : end+1])
	if !ok {
		return nil
	}
	replacement := append(append(append([]rune{}, t[start].GetCh(s)...), '-'), t[end].GetCh(s)...)
	return &lint.Lint{Span: span, LintKind: lint.Punctuation, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCase(replacement, span.GetContent(s))}, Message: "Hyphenate this mouse command.", Priority: 40}
}
