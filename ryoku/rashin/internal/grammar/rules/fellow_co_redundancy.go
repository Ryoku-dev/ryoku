package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type FellowCoRedundancy struct{ expression expr.Expr }

func NewFellowCoRedundancy() *FellowCoRedundancy {
	return &FellowCoRedundancy{expr.NewSequenceExprAco("fellow").TWs().ThenAnyOf([]expr.Expr{expr.NewSequenceExprAco("co").TWsH().TSet([]string{"admin", "admins", "chair", "chairs", "conspirator", "conspirators", "equal", "equals", "founder", "founders", "host", "hosts", "intern", "interns", "leader", "leaders", "organiser", "organisers", "organizer", "organizers", "pilot", "pilots", "worker", "workers"}), expr.NewSequenceExprWordSet([]string{"coadmin", "coadmins", "cochair", "cochairs", "coconspirator", "coconspirators", "coequal", "coequals", "cofounder", "cofounders", "cohost", "cohosts", "cointern", "cointerns", "coorganiser", "coorganisers", "coorganizer", "coorganizers", "coleader", "coleaders", "copilot", "copilots", "coworker", "coworkers"})})}
}
func (r *FellowCoRedundancy) Expr() expr.Expr                  { return r.expression }
func (r *FellowCoRedundancy) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*FellowCoRedundancy) Description() string {
	return "Corrects redundant use of `fellow` with `co-`."
}
func (r *FellowCoRedundancy) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) < 3 {
		return nil
	}
	cw := t[2:]
	var co, worker []rune
	switch len(cw) {
	case 1:
		ch := cw[0].GetCh(s)
		if len(ch) < 2 {
			return nil
		}
		co, worker = ch[:2], ch[2:]
	case 3:
		co, _ = doc.GetCh(cw[:2], s)
		worker, _ = doc.GetCh(cw[2:], s)
	default:
		return nil
	}
	fellow, _ := doc.GetCh(t[:2], s)
	span, _ := doc.Span(t)
	a := append(append([]rune(nil), co...), worker...)
	b := append(append([]rune(nil), fellow...), worker...)
	return &lint.Lint{Span: span, LintKind: lint.Redundancy, Suggestions: []lint.Suggestion{lint.ReplaceWith(a), lint.ReplaceWith(b)}, Message: "Using `fellow` with `co` is redundant.", Priority: 127}
}
