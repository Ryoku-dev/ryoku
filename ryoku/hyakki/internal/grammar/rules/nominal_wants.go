package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/pos"
	"ryoku-rashin/internal/grammar/text"
)

type NominalWants struct{ expression expr.Expr }

func NewNominalWants() *NominalWants {
	return &NominalWants{expression: expr.NewSequenceExprWith(func(t doc.Token, s []rune) bool {
		return t.Kind.IsPronoun() && t.Kind.IsUpos(pos.PRON) && !text.EqAnyIgnoreASCIICaseStr(t.GetCh(s), []string{"that", "me", "us", "him", "her", "it", "them", "who"})
	}).TWs().TSet([]string{"wont", "wonts", "want", "wants"})}
}
func (r *NominalWants) Expr() expr.Expr                  { return r.expression }
func (r *NominalWants) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *NominalWants) Description() string {
	return "Ensures you use the correct `want` / `wants` after a nominal."
}
func (r *NominalWants) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	subject := toks[0]
	person, ok := subject.Kind.GetPronounPerson()
	if !ok {
		person = dict.Third
	}
	replacement := "want"
	if person == dict.Third && !subject.Kind.IsPluralNominal() {
		replacement = "wants"
	}
	span := toks[len(toks)-1].Span
	if text.EqCh(span.GetContent(src), []rune(replacement)) {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Miscellaneous, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr(replacement, span.GetContent(src))}, Message: fmt.Sprintf("Did you mean `%s`?", replacement), Priority: 55}
}
