package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type TakePrideIn struct{ expression expr.Expr }

func NewTakePrideIn() *TakePrideIn {
	return &TakePrideIn{expression: expr.ButNot(expr.NewSequenceExprWordSet([]string{"take", "taken", "takes", "taking", "took"}).TWs().ThenWordSeq([]string{"pride", "of"}), expr.NewSequenceExprAnything().TAny().TAny().TAny().TAny().TWs().TAco("place"))}
}
func (r *TakePrideIn) Expr() expr.Expr                  { return r.expression }
func (r *TakePrideIn) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*TakePrideIn) Description() string                { return "Corrects `take pride of` to `take pride in`." }
func (r *TakePrideIn) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	span := toks[len(toks)-1].Span
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Usage
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("in", span.GetContent(src))}
	result.Message = "This idiom uses the preposition `in` rather than `of`."
	result.Priority = 127
	return &result
}
