package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type AllowTo struct{ expression expr.Expr }

func NewAllowTo() *AllowTo {
	return &AllowTo{expression: expr.NewSequenceExprWordSet([]string{"allow", "allowing", "allows"}).TWs().TAco("to").ThenOptional(expr.NewSequenceExpr().TWs().ThenAdverb()).TWs().ThenAnyWord()}
}
func (r *AllowTo) Expr() expr.Expr                  { return r.expression }
func (r *AllowTo) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*AllowTo) Description() string                { return "Flags erroneous usage of `allow to` without a subject." }
func (r *AllowTo) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	span, _ := doc.Span(toks)
	allow := toks[0].GetStr(src)
	rlt := lint.NewLint()
	rlt.Span = span
	rlt.LintKind = lint.Grammar
	rlt.Message = fmt.Sprintf("For correct usage, either add a subject between `%s` and `to` (e.g., `%s someone to do`) or use the present participle (e.g., `%s doing`).", allow, allow, allow)
	return &rlt
}
