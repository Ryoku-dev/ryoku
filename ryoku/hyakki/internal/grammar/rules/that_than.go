package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type ThatThan struct{ expression expr.Expr }

func NewThatThan() *ThatThan {
	return &ThatThan{expression: expr.NewSequenceExpr().ThenKindExcept(doc.TokenKind.IsComparativeAdjective, []string{"better", "later", "number"}).TWs().TAco("that").TWs().ThenWordExcept([]string{"way"})}
}
func (r *ThatThan) Expr() expr.Expr                  { return r.expression }
func (r *ThatThan) Description() string              { return "Corrects the typo `that` to `than` in comparisons." }
func (r *ThatThan) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ThatThan) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Typo
	result.Priority = 31
	if len(toks) != 5 {
		return nil
	}
	result.Span = toks[2].Span
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("than", toks[2].GetCh(src))}
	result.Message = "This looks like a comparison that should use `than` rather than `that`."
	return &result
}
