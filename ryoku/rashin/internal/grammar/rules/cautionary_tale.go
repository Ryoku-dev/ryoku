package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

// CautionaryTale corrects the homophone confusion between "tale" (story) and
// "tail" (appendage) in common phrases like "cautionary tale" and "inspirational tale".
type CautionaryTale struct{ expression expr.Expr }

func NewCautionaryTale() *CautionaryTale {
	return &CautionaryTale{expression: expr.NewSequenceExprWordSet([]string{"cautionary", "inspirational"}).TWs().TAco("tail")}
}
func (r *CautionaryTale) Expr() expr.Expr                  { return r.expression }
func (r *CautionaryTale) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *CautionaryTale) Description() string {
	return "Corrects confusion between `tale` (story) and `tail` (appendage) in common phrases."
}
func (r *CautionaryTale) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	span := toks[len(toks)-1].Span
	return &lint.Lint{Span: span, LintKind: lint.Miscellaneous, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("tale", span.GetContent(src))}, Message: "Did you mean `tale` (story)?", Priority: 31}
}
