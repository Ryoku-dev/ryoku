package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type AnotherThingComing struct{ expression expr.Expr }

func NewAnotherThingComing() *AnotherThingComing {
	return &AnotherThingComing{expression: expr.NewSequenceExprWordSet([]string{"had", "has", "have", "got"}).ThenFixedPhrase(" another think coming")}
}
func (r *AnotherThingComing) Expr() expr.Expr                  { return r.expression }
func (r *AnotherThingComing) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*AnotherThingComing) Description() string {
	return "Though `another think coming` is the original phrase, `another thing coming` is now more common."
}
func (r *AnotherThingComing) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span, ok := doc.Span(t[2:])
	if !ok {
		return nil
	}
	full, _ := doc.Span(t)
	return &lint.Lint{Span: span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("another thing coming", s[full.Start:full.End])}, Message: "Corrects `another think coming` to `another thing coming`", Priority: 63}
}
