package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type BellowBelow struct{ expression expr.Expr }

func NewBellowBelow() *BellowBelow {
	return &BellowBelow{expression: expr.NewSequenceExprAco("bellow").TWs().ThenAnyOf([]expr.Expr{expr.AsExpr(func(t doc.Token, _ []rune) bool { return t.Kind.IsDeterminer() }), expr.NewSequenceExprNumber(), expr.NewSpelledNumberExpr()})}
}
func (r *BellowBelow) Expr() expr.Expr                  { return r.expression }
func (r *BellowBelow) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*BellowBelow) Description() string {
	return "Looks for `bellow` wrongly used when `below` is intended."
}
func (r *BellowBelow) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	span := toks[0].Span
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Malapropism
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("below", span.GetContent(src))}
	result.Message = "`Bellow` means to shout. You probably mean `below`."
	result.Priority = 127
	return &result
}
