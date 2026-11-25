package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type NailInCoffin struct{ expression expr.Expr }

func NewNailInCoffin() *NailInCoffin {
	e := expr.NewSequenceExprWordSet([]string{"final", "last"}).TWs().TSet([]string{"nail", "nails"}).TWs().TAco("on").TWs().ThenDeterminer().TWs().TSet([]string{"coffin", "coffins"})
	return &NailInCoffin{expression: e}
}
func (r *NailInCoffin) Expr() expr.Expr { return r.expression }
func (r *NailInCoffin) Description() string {
	return "Corrects `nail on the coffin` to `nail in the coffin`"
}
func (r *NailInCoffin) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *NailInCoffin) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	tok := lint.FindTheOnlyTokenMatching(t, s, func(t *doc.Token, s []rune) bool { return text.EqStr(t.GetCh(s), "on") })
	if tok == nil {
		return nil
	}
	l := lint.NewLint()
	l.Span = tok.Span
	l.LintKind = lint.Usage
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("in", tok.GetCh(s))}
	l.Message = "This idiom uses the preposition `in` rather than `on`"
	return &l
}
