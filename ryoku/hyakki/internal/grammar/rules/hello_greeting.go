package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type HelloGreeting struct{ expression expr.Expr }

func NewHelloGreeting() *HelloGreeting {
	return &HelloGreeting{expression: expr.NewSequenceExprWith(expr.NewAnchorStart()).ThenOptional(expr.NewSequenceExpr().TWs()).ThenOptional(expr.NewSequenceExpr().ThenQuote().ThenOptional(expr.NewSequenceExpr().TWs())).TAco("halo")}
}
func (r *HelloGreeting) Expr() expr.Expr { return r.expression }
func (r *HelloGreeting) Description() string {
	return "Encourages greeting someone with `hello` instead of the homophone `halo`."
}
func (r *HelloGreeting) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *HelloGreeting) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	word, ok := doc.FirstWord(t)
	if !ok {
		return nil
	}
	return &lint.Lint{Span: word.Span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("hello", word.GetCh(s))}, Message: "Prefer `hello` as a greeting; `halo` refers to the optical effect.", Priority: 31}
}
