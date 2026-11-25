package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type ThatWhich struct{ expression expr.Expr }

func NewThatWhich() *ThatWhich {
	e := expr.NewWordExprGroup()
	p := expr.NewSequenceExprWordSeq([]string{"that", "that"})
	e.Add("that", p)
	e.Add("That", p)
	return &ThatWhich{expression: e}
}
func (r *ThatWhich) Expr() expr.Expr                  { return r.expression }
func (r *ThatWhich) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ThatWhich) Description() string {
	return "Repeating the word \"that\" is often redundant. The phrase `that which` is easier to read."
}
func (r *ThatWhich) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Repetition, Suggestions: []lint.Suggestion{lint.ReplaceWith([]rune(toks[0].GetStr(src) + " which"))}, Message: "“that that” sometimes means “that which”, which is clearer.", Priority: 126}
}
