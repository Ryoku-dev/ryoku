package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type PreferSnuck struct{ expression expr.Expr }

func NewPreferSnuck() *PreferSnuck                      { return &PreferSnuck{expr.AsExpr(expr.NewWord("sneaked"))} }
func (r *PreferSnuck) Expr() expr.Expr                  { return r.expression }
func (r *PreferSnuck) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*PreferSnuck) Description() string                { return "Prefer `snuck` over `sneaked`." }
func (*PreferSnuck) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return sneakedSnuckToLint(t, s, "snuck", "sneaked")
}

type PreferSneaked struct{ expression expr.Expr }

func NewPreferSneaked() *PreferSneaked                    { return &PreferSneaked{expr.AsExpr(expr.NewWord("snuck"))} }
func (r *PreferSneaked) Expr() expr.Expr                  { return r.expression }
func (r *PreferSneaked) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*PreferSneaked) Description() string                { return "Prefer `sneaked` over `snuck`." }
func (*PreferSneaked) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return sneakedSnuckToLint(t, s, "sneaked", "snuck")
}
func sneakedSnuckToLint(toks []doc.Token, src []rune, target, source string) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	span := toks[0].Span
	chars := span.GetContent(src)
	if !text.EqStr(chars, source) {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Usage
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(target, chars)}
	result.Message = fmt.Sprintf("Use `%s` instead of `%s`.", target, source)
	return &result
}
