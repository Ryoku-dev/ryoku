package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type PreferPled struct{ expression expr.Expr }

func NewPreferPled() *PreferPled                       { return &PreferPled{expression: expr.NewWord("pleaded")} }
func (r *PreferPled) Expr() expr.Expr                  { return r.expression }
func (r *PreferPled) Description() string              { return "Prefer `pled` over `pleaded`." }
func (r *PreferPled) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *PreferPled) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 || !text.EqCh(t[0].GetCh(s), []rune("pleaded")) {
		return nil
	}
	return &lint.Lint{Span: t[0].Span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("pled", t[0].GetCh(s))}, Message: "Use `pled` instead of `pleaded`.", Priority: 127}
}

type PreferPleaded struct{ expression expr.Expr }

func NewPreferPleaded() *PreferPleaded                    { return &PreferPleaded{expression: expr.NewWord("pled")} }
func (r *PreferPleaded) Expr() expr.Expr                  { return r.expression }
func (r *PreferPleaded) Description() string              { return "Prefer `pleaded` over `pled`." }
func (r *PreferPleaded) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *PreferPleaded) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 || !text.EqCh(t[0].GetCh(s), []rune("pled")) {
		return nil
	}
	return &lint.Lint{Span: t[0].Span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("pleaded", t[0].GetCh(s))}, Message: "Use `pleaded` instead of `pled`.", Priority: 127}
}
