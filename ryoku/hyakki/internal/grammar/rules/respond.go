package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type Respond struct{ expression expr.Expr }

func NewRespond() *Respond {
	m := expr.NewExprMap[int]()
	helper := expr.Predicate(func(t doc.Token, s []rune) bool {
		if t.Kind.IsAuxiliaryVerb() {
			return true
		}
		if !t.Kind.IsVerb() {
			return false
		}
		switch strings.ToLower(t.GetStr(s)) {
		case "do", "did", "does", "won't", "don't", "didn't", "doesn't":
			return true
		}
		return false
	})
	m.Insert(expr.NewSequenceExpr().ThenNominal().TWs().Then(helper).TWs().TAco("response"), 4)
	m.Insert(expr.NewSequenceExpr().ThenNominal().TWs().Then(helper).TWs().ThenAdverb().TWs().TAco("response"), 6)
	return &Respond{expression: m}
}
func (r *Respond) Expr() expr.Expr { return r.expression }
func (r *Respond) Description() string {
	return "Flags uses of the noun `response` where the verb `respond` is needed after an auxiliary."
}
func (r *Respond) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *Respond) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	idx, ok := r.expression.(*expr.ExprMap[int]).Lookup(0, t, s)
	if !ok || idx >= len(t) {
		return nil
	}
	return &lint.Lint{Span: t[idx].Span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("respond", t[idx].GetCh(s))}, Message: "Use the verb `respond` here.", Priority: 40}
}
