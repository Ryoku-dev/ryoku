package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type Akimbo struct{ expression expr.Expr }

func NewAkimbo() *Akimbo {
	return &Akimbo{expression: expr.ButNot(expr.NewSequenceExprWordSet([]string{"arm", "arms", "elbows", "all", "fingers", "hand", "hands", "legs", "limbs", "stand", "standing", "stands", "stood"}).TWsH().TAco("a").TWsH().TAco("kimbo"), expr.NewSequenceExprAnything().TAny().TAny().TAny().TAny().TWs().TSet([]string{"slice"}))}
}
func (r *Akimbo) Expr() expr.Expr                  { return r.expression }
func (r *Akimbo) Description() string              { return "Corrects `a kimbo` to `akimbo`." }
func (r *Akimbo) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *Akimbo) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) < 5 {
		return nil
	}
	span, _ := doc.Span(t[2:5])
	fix := append(append([]rune{}, t[2].GetCh(s)...), t[4].GetCh(s)...)
	return &lint.Lint{Span: span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.ReplaceWith(fix)}, Message: "Did you mean `akimbo` (standing with both hands on hips)?", Priority: 127}
}
