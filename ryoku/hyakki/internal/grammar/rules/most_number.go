package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type MostNumber struct{ expression expr.Expr }

func NewMostNumber() *MostNumber {
	e := expr.NewAll([]expr.Expr{expr.NewSequenceExpr().TAco("most").TWs().TSet([]string{"amount", "number"}), expr.NewSequenceExprAnything().ThenAnything().ThenAnything().ThenAnything().TAco("of")})
	return &MostNumber{expression: e}
}
func (r *MostNumber) Expr() expr.Expr                  { return r.expression }
func (r *MostNumber) Description() string              { return "Corrects `most number` and `most amount`" }
func (r *MostNumber) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *MostNumber) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span, _ := doc.Span(t[:3])
	noun := t[2].GetStr(s)
	supers := []string{"highest", "largest", "maximum"}
	if noun == "amount" {
		supers = []string{"largest", "greatest", "maximum"}
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Miscellaneous
	for _, v := range supers {
		l.Suggestions = append(l.Suggestions, lint.ReplaceWithMatchCaseStr(v+" "+noun, span.GetContent(s)))
	}
	l.Message = "`Most` is not standard before `" + noun + "`."
	l.Priority = 31
	return &l
}
