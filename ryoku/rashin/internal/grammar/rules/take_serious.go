package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

// TakeSerious corrects "take X serious" to "take X seriously".
//
// It identifies the common mistake of using the adjective "serious" instead
// of the adverb "seriously" in phrases like "take it serious".
type TakeSerious struct{ expression expr.Expr }

// NewTakeSerious matches any form of "take", followed by a nominal phrase,
// and ending with "serious".
func NewTakeSerious() *TakeSerious {
	return &TakeSerious{expression: expr.NewSequenceExprWordSet([]string{"take", "taken", "takes", "taking", "took"}).TWs().Then(expr.NewNominalPhrase()).TWs().TAco("serious")}
}
func (r *TakeSerious) Expr() expr.Expr                  { return r.expression }
func (r *TakeSerious) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*TakeSerious) Description() string {
	return "Ensures the correct use of the adverb `seriously` instead of the adjective `serious` in phrases like `take it seriously`."
}
func (r *TakeSerious) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok || len(toks) < 2 {
		return nil
	}
	prefix, ok := doc.Span(toks[:len(toks)-1])
	if !ok {
		return nil
	}
	value := append(append([]rune(nil), prefix.GetContent(src)...), []rune("seriously")...)
	return &lint.Lint{Span: span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCase(value, span.GetContent(src))}, Message: "Take seriously", Priority: 63}
}
