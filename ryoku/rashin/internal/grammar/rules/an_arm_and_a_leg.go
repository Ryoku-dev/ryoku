package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type AnArmAndALeg struct{ expression expr.Expr }

func NewAnArmAndALeg() *AnArmAndALeg {
	article := func() *expr.SequenceExpr { return expr.NewSequenceExprWordSet([]string{"a", "an", "the"}).TWs() }
	limbs := func(a, b []string) *expr.SequenceExpr {
		return expr.NewSequenceExprOptional(article()).TSet(a).TWs().TAco("and").TWs().ThenOptional(article()).TSet(b)
	}
	return &AnArmAndALeg{expr.ButNot(expr.NewSequenceExprWordSet([]string{"charge", "charged", "charges", "charging", "cost", "costed", "costing", "costs", "paid", "pay", "payed", "paying", "pays"}).TWs().ThenAnyOf([]expr.Expr{limbs([]string{"arm", "arms"}, []string{"leg", "legs"}), limbs([]string{"leg", "legs"}, []string{"arm", "arms"})}), expr.NewSequenceExprAnything().TAny().ThenWordSeq([]string{"an", "arm", "and", "a", "leg"}))}
}
func (r *AnArmAndALeg) Expr() expr.Expr                  { return r.expression }
func (r *AnArmAndALeg) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*AnArmAndALeg) Description() string {
	return "Corrects wrong variants of the idiom `an arm and a leg`."
}
func (r *AnArmAndALeg) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) < 3 {
		return nil
	}
	span, ok := doc.Span(t[2:])
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("an arm and a leg", span.GetContent(s))}, Message: "Use the standard idiom `an arm and a leg` instead.", Priority: 127}
}
