package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

// TheHowWhy suggests removing `the` when followed by how/why/who/when/what,
// skipping cases like `how to` and `who's who`.
type TheHowWhy struct{ expression expr.Expr }

func NewTheHowWhy() *TheHowWhy {
	return &TheHowWhy{expr.NewFirstMatchOf([]expr.Expr{expr.NewSequenceExprAco("the").TWs().TAco("how").ThenUnless(expr.NewSequenceExprWhitespace().TAco("to")), expr.NewSequenceExprAco("the").TWs().TAco("who").ThenUnless(expr.NewSequenceExprWhitespace().TAco("'s").TWs().TAco("who")), expr.NewSequenceExprAco("the").TWs().TAco("why"), expr.NewSequenceExprAco("the").TWs().TAco("when"), expr.NewSequenceExprAco("the").TWs().TAco("what")})}
}
func (r *TheHowWhy) Expr() expr.Expr                  { return r.expression }
func (r *TheHowWhy) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*TheHowWhy) Description() string {
	return "Removes the extra `the` from expressions like `the how`, skipping `how to` and `who's who`."
}
func (r *TheHowWhy) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) < 3 {
		return nil
	}
	span, _ := doc.Span(t[:2])
	question := t[2].GetStr(s)
	return &lint.Lint{Span: span, LintKind: lint.Miscellaneous, Suggestions: []lint.Suggestion{lint.Remove()}, Message: fmt.Sprintf("Remove `the` before `%s`. In most contexts, `%s` alone is clearer.", question, question), Priority: 31}
}
