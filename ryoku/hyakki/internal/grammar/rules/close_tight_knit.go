package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type CloseTightKnit struct{ expression expr.Expr }

func NewCloseTightKnit() *CloseTightKnit {
	return &CloseTightKnit{expression: expr.NewSequenceExprWordSet([]string{"close", "closely", "tight", "tightly"}).TWsH().TAco("nit")}
}
func (r *CloseTightKnit) Expr() expr.Expr                  { return r.expression }
func (r *CloseTightKnit) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*CloseTightKnit) Description() string {
	return "Corrects `close-nit` and `tight-nit` to `close-knit` and `tight-knit`."
}
func (r *CloseTightKnit) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	span := toks[2].Span
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Malapropism
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("knit", span.GetContent(src))}
	sep := " "
	if toks[1].Kind.IsHyphen() {
		sep = "-"
	}
	result.Message = "A `nit` is a louse egg. The correct idiom is `tight" + sep + "knit`."
	result.Priority = 127
	return &result
}
