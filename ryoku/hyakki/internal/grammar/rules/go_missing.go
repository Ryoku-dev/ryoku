package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"strings"
)

type GoMissing struct{ expression expr.Expr }

func NewGoMissing() *GoMissing {
	return &GoMissing{expression: expr.ButNot(expr.NewSequenceExprWordSet([]string{"become", "became", "becomes", "becoming"}).TWs().TAco("missing"), expr.NewSequenceExprAnything().TAny().TAny().TWs().Then(func(t doc.Token, s []rune) bool {
		return t.Kind.IsPluralNoun() || text.EqAnyIgnoreASCIICaseStr(t.GetCh(s), []string{"child", "person"})
	}))}
}
func (r *GoMissing) Expr() expr.Expr                  { return r.expression }
func (r *GoMissing) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *GoMissing) Description() string              { return "Corrects `become missing` to `go missing`." }
func (r *GoMissing) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	ch := toks[0].GetCh(src)
	var corrections []string
	switch strings.ToLower(string(ch)) {
	case "became":
		corrections = []string{"went"}
	case "become":
		corrections = []string{"go", "gone"}
	case "becomes":
		corrections = []string{"goes"}
	case "becoming":
		corrections = []string{"going"}
	default:
		return nil
	}
	return &lint.Lint{Span: toks[0].Span, LintKind: lint.Usage, Suggestions: lint.ToReplaceSuggestions(corrections, ch), Message: "The idiomatic expression is `go missing`.", Priority: 127}
}
