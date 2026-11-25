package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type Catch22 struct{ expression expr.Expr }

func NewCatch22() *Catch22 {
	e := expr.NewSequenceExprWordSet([]string{"cache", "cash"}).ThenAnyOf([]expr.Expr{expr.NewSequenceExpr().TWsH().Then(func(t doc.Token, s []rune) bool { return t.Kind.IsNumber() && text.EqStr(t.GetCh(s), "22") }), expr.NewSequenceExpr().TWs().TAco("twenty").TWsH().TAco("two")})
	return &Catch22{expression: e}
}
func (r *Catch22) Expr() expr.Expr { return r.expression }
func (r *Catch22) Description() string {
	return "Corrects mistakenly using similar-sounding words in the idiom `catch 22`."
}
func (r *Catch22) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *Catch22) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	l := lint.NewLint()
	l.Span = t[0].Span
	l.LintKind = lint.Malapropism
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("catch", t[0].GetCh(s))}
	l.Message = "This idiom uses 'catch' instead of 'cache' or 'cash'."
	return &l
}
