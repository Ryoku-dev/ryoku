package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type CraveFor struct{ expression expr.Expr }

func NewCraveFor() *CraveFor {
	return &CraveFor{expression: expr.NewFirstMatchOf([]expr.Expr{expr.NewSequenceExprWordSet([]string{"crave", "craved", "craves"}).TWs().TAco("for"), expr.ButNot(expr.NewSequenceExprAnyOf([]expr.Expr{expr.NewInflectionOfBe(), expr.AsExpr(expr.NewWordSet([]string{"i'm", "we're", "you're", "he's", "she's", "it's", "they're"}))}).TWs().TAco("craving").TWs().TAco("for"), expr.NewWord("being"))})}
}
func (r *CraveFor) Expr() expr.Expr                  { return r.expression }
func (r *CraveFor) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *CraveFor) Description() string              { return "There should be no `for` after the verb `crave`." }
func (r *CraveFor) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span, ok := doc.Span(t[len(t)-2:])
	if !ok {
		return nil
	}
	rlt := lint.NewLint()
	rlt.Span = span
	rlt.LintKind = lint.Usage
	rlt.Suggestions = []lint.Suggestion{lint.Remove()}
	rlt.Message = "The verb `crave` should not be followed by `for`."
	return &rlt
}
