package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ImplementIn struct{ expression expr.Expr }

func NewImplementIn() *ImplementIn {
	e := expr.NewSequenceExprWordSet([]string{"implement", "implemented", "implementing", "implements"}).TWs().ThenAnyOf([]expr.Expr{expr.NewSequenceExprAco("into"), expr.NewSequenceExprAnyWord().TWs().TAco("into")})
	return &ImplementIn{expression: e}
}
func (r *ImplementIn) Expr() expr.Expr { return r.expression }
func (r *ImplementIn) Description() string {
	return "Corrects nonstandard `implement into` to `implement in`."
}
func (r *ImplementIn) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ImplementIn) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	var mid *doc.Token
	if len(t) == 5 {
		mid = &t[2]
	} else if len(t) != 3 {
		return nil
	}
	itok := t[len(t)-1]
	if t[0].Kind.IsNoun() && mid != nil && mid.Kind.IsVerb() && !mid.Kind.IsNoun() {
		return nil
	}
	if (mid == nil || text.EqStr(mid.GetCh(s), "the")) && itok.GetStr(s) == "Into" {
		return nil
	}
	l := lint.NewLint()
	l.Span = itok.Span
	l.LintKind = lint.Usage
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("in", itok.GetCh(s))}
	l.Message = "Prefer `in` over `into` in this usage."
	return &l
}
