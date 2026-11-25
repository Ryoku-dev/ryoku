package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/weir"
)

type ChockFull struct{ expression expr.Expr }

func NewChockFull() *ChockFull {
	e, err := weir.WeirExprToExpr("[chalk, choke, chocked, chucked, choked][( ), -]full")
	if err != nil {
		panic(err)
	}
	return &ChockFull{expression: e}
}
func (r *ChockFull) Expr() expr.Expr { return r.expression }
func (r *ChockFull) Description() string {
	return "Flags common soundalikes of \"chock-full\" and makes sure they're hyphenated."
}
func (r *ChockFull) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ChockFull) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span, ok := doc.Span(t)
	if !ok {
		return nil
	}
	msg := "The standard term is \"chock-full\""
	if t[1].Kind.IsWhitespace() {
		msg += ", and it should be hyphenated"
	}
	msg += "."
	return &lint.Lint{Span: span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("chock-full", span.GetContent(s))}, Message: msg, Priority: 126}
}
