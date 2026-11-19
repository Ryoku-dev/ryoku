package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type OpenTheLight struct{ expression expr.Expr }

func NewOpenTheLight() *OpenTheLight {
	base := expr.NewSequenceExprWordSet([]string{"open", "opens", "opened", "opening"}).TWs().ThenDeterminer().TWs().ThenWordSet([]string{"air conditioner", "air conditioning", "aircon", "cellphone", "fan", "handphone", "heater", "heating", "lamp", "light", "lights", "radio", "telephone", "television", "TV"})
	return &OpenTheLight{expression: expr.NewLongestMatchOf([]expr.Expr{base, expr.NewSequenceExprWith(base).TWs().ThenNoun()})}
}
func (r *OpenTheLight) Expr() expr.Expr                  { return r.expression }
func (r *OpenTheLight) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *OpenTheLight) Description() string {
	return "Corrects using `open` instead of `turn on` or `switch on`"
}
func (r *OpenTheLight) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 7 && (!t[len(t)-3].Kind.IsPluralNoun() || !t[len(t)-1].Kind.IsNoun()) {
		return nil
	}
	if len(t) == 0 {
		return nil
	}
	verb := t[0].GetStr(s)
	turnEnding, switchEnding := "", ""
	if strings.HasSuffix(verb, "ing") {
		turnEnding = "ing"
		switchEnding = "ing"
	} else if strings.HasSuffix(verb, "ed") {
		turnEnding = "ed"
		switchEnding = "ed"
	} else if strings.HasSuffix(verb, "s") {
		turnEnding = "s"
		switchEnding = "es"
	}
	span, ok := doc.Span(t)
	if !ok {
		return nil
	}
	template := s[span.Start:span.End]
	return &lint.Lint{Span: t[0].Span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("turn"+turnEnding+" on", template), lint.ReplaceWithMatchCaseStr("switch"+switchEnding+" on", template)}, Message: "Are you accessing the device's internals or `turning` it `on`?", Priority: 63}
}
