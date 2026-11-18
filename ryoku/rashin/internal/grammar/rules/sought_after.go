package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type SoughtAfter struct{ expression expr.Expr }

func NewSoughtAfter() *SoughtAfter {
	return &SoughtAfter{expression: expr.NewSequenceExprAnyOf([]expr.Expr{expr.NewSequenceExpr().ThenKindExcept(doc.TokenKind.IsAdverb, []string{"always", "maybe", "not", "perhaps"}), expr.NewSequenceExprWordSet([]string{"abit", "are", "bit", "of", "quiet"})}).TWs().TAco("sort").TWsH().TAco("after")}
}
func (r *SoughtAfter) Expr() expr.Expr                  { return r.expression }
func (r *SoughtAfter) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *SoughtAfter) Description() string              { return "Correct `sort after` to `sought after`" }
func (r *SoughtAfter) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 3 {
		return nil
	}
	result := lint.NewLint()
	result.Span = toks[2].Span
	result.LintKind = lint.Eggcorn
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("sought", toks[2].GetCh(src))}
	result.Message = "The correct word in this context is `sought`."
	result.Priority = 63
	return &result
}
