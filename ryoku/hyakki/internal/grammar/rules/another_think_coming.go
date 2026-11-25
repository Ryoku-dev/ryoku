package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

// Both `another thing coming` and `another think coming` are correct, but `another think coming` is the original.
type AnotherThinkComing struct{ expression expr.Expr }

func NewAnotherThinkComing() *AnotherThinkComing {
	return &AnotherThinkComing{expression: expr.NewSequenceExprWordSet([]string{"had", "has", "have", "got"}).ThenFixedPhrase(" another thing coming")}
}
func (r *AnotherThinkComing) Expr() expr.Expr                  { return r.expression }
func (r *AnotherThinkComing) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *AnotherThinkComing) Description() string {
	return "Though `another thing coming` is now more common, `another think coming` is the original phrase."
}
func (r *AnotherThinkComing) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks[2:])
	if !ok {
		return nil
	}
	all, _ := doc.Span(toks)
	return &lint.Lint{Span: span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("another think coming", all.GetContent(src))}, Message: "Corrects `another thing coming` to `another think coming`", Priority: 63}
}
