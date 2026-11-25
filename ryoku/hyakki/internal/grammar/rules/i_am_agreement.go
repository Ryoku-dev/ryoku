package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type IAmAgreement struct{ expression expr.Expr }

func NewIAmAgreement() *IAmAgreement {
	iAre := expr.NewFixedPhraseFromPhrase("I are")
	return &IAmAgreement{expr.NewFirstMatchOf([]expr.Expr{expr.NewSequenceExprWith(expr.NewAnchorStart()).Then(iAre), expr.NewSequenceExpr().ThenWordExcept([]string{"and"}).TWs().Then(iAre)})}
}
func (r *IAmAgreement) Expr() expr.Expr                  { return r.expression }
func (r *IAmAgreement) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*IAmAgreement) Description() string                { return "Corrects `I are` to `I am`." }
func (r *IAmAgreement) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) < 3 {
		return nil
	}
	span, ok := doc.Span(t[len(t)-3:])
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Agreement, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("I am", span.GetContent(s))}, Message: "The first-person singular pronoun `I` requires the verb form `am`; `are` belongs to second-person or plural contexts.", Priority: 31}
}
