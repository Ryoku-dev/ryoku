package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type AllHellBreakLoose struct{ expression expr.Expr }

func NewAllHellBreakLoose() *AllHellBreakLoose {
	e := expr.NewSequenceExprWordSeq([]string{"all", "hell"}).TWs().TSet([]string{"break", "breaking", "breaks", "broke", "broken"}).TWs().TAco("out")
	return &AllHellBreakLoose{expression: e}
}
func (r *AllHellBreakLoose) Expr() expr.Expr { return r.expression }
func (r *AllHellBreakLoose) Description() string {
	return "Corrects forms of `all hell breaks out` to `all hell breaks loose`."
}
func (r *AllHellBreakLoose) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *AllHellBreakLoose) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	tok := t[len(t)-1]
	l := lint.NewLint()
	l.Span = tok.Span
	l.LintKind = lint.Eggcorn
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("loose", tok.GetCh(s))}
	l.Message = "The correct idiom is `all hell breaks loose`."
	return &l
}
