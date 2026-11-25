package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type NoMatchFor struct{ expression expr.Expr }

func NewNoMatchFor() *NoMatchFor {
	return &NoMatchFor{expression: expr.NewSequenceExprWith(expr.NewFirstMatchOf([]expr.Expr{expr.NewInflectionOfBe(), expr.NewWordSet([]string{"I'm", "we're", "you're", "he's", "she's", "it's", "they're", "Im", "were", "youre", "hes", "shes", "its", "theyre"})})).ThenWhitespace().TAco("no").ThenWhitespace().TAco("match").ThenWhitespace().ThenPreposition()}
}
func (r *NoMatchFor) Expr() expr.Expr                  { return r.expression }
func (r *NoMatchFor) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *NoMatchFor) Description() string              { return "No match for" }
func (r *NoMatchFor) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 3 || text.EqStr(toks[len(toks)-1].GetCh(src), "for") {
		return nil
	}
	span, ok := doc.Span(toks[2:])
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("no match for", span.GetContent(src))}, Message: "If you mean the idiom, it's `no match for`.", Priority: 55}
}
