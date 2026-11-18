package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ObsessPreposition struct{ expression expr.Expr }

func NewObsessPreposition() *ObsessPreposition {
	return &ObsessPreposition{expression: expr.NewSequenceExprWordSet([]string{"obsess", "obsessed", "obsesses", "obsessing"}).TWs().ThenPreposition()}
}
func (r *ObsessPreposition) Expr() expr.Expr                  { return r.expression }
func (r *ObsessPreposition) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*ObsessPreposition) Description() string {
	return "Ensures valid prepositions are used with `obsess`"
}
func (r *ObsessPreposition) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	verb := toks[0].GetCh(src)
	span := toks[len(toks)-1].Span
	prep := span.GetContent(src)
	ed := text.EndsWithIgnoreASCIICaseStr(verb, "ed")
	if text.EqStr(prep, "over") || (ed && text.EqStr(prep, "with")) {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Usage
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("over", prep)}
	l.Message = "Use 'over' instead of '" + string(prep) + "'."
	if ed {
		l.Suggestions = append(l.Suggestions, lint.ReplaceWithMatchCaseStr("with", prep))
		l.Message = "For `excessively preoccupied with` use `obsessed with`. For `paid close attention to details` use `obsessed over`"
	}
	return &l
}
