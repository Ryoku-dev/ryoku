package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type MostOfTheTimes struct{ expression expr.Expr }

func NewMostOfTheTimes() *MostOfTheTimes {
	return &MostOfTheTimes{expr.NewSequenceExprAnyOf([]expr.Expr{expr.NewFixedPhraseFromPhrase("a lot"), expr.AsExpr(expr.NewWord("most"))}).TWs().ThenFixedPhrase("of the times")}
}
func (r *MostOfTheTimes) Expr() expr.Expr                  { return r.expression }
func (r *MostOfTheTimes) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*MostOfTheTimes) Description() string {
	return "Corrects `a lot of the times` and `most of the times` to use singular `time`."
}
func (r *MostOfTheTimes) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	w := t[len(t)-1]
	return &lint.Lint{Span: w.Span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("time", w.GetCh(s))}, Message: "Singular `time` is usually the correct form in this context.", Priority: 32}
}
