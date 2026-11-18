package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type FarBeIt struct{ expression expr.Expr }

func NewFarBeIt() *FarBeIt {
	return &FarBeIt{expression: expr.NewSequenceExprAco("far").TWs().TAco("be").TWs().TAco("it").TWs().ThenWordExcept([]string{"from"})}
}
func (r *FarBeIt) Expr() expr.Expr                  { return r.expression }
func (r *FarBeIt) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*FarBeIt) Description() string {
	return "Flags misuse of `far be it` and suggests using `from` when it is followed by `for`"
}
func (r *FarBeIt) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	tok := t[len(t)-1]
	l := lint.NewLint()
	l.Span = tok.Span
	l.LintKind = lint.Usage
	if text.EqStr(tok.GetCh(s), "for") {
		l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("from", tok.GetCh(s))}
		l.Message = "`Far be it for` is a common error for `far be it from`"
	} else {
		l.Message = "The correct usage of the idiom is `far be it from` [someone] to [do something]. Try to rephrase the sentence."
	}
	return &l
}
