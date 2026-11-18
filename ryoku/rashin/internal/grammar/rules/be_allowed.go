package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type BeAllowed struct{ expression *expr.ExprMap[int] }

func NewBeAllowed() *BeAllowed {
	e := expr.NewExprMap[int]()
	e.Insert(expr.NewSequenceExpr().TAco("will").TWs().ThenWordSet([]string{"not"}).TWs().TAco("allowed").TWs().TAco("to").TWs().ThenVerb(), 4)
	e.Insert(expr.NewSequenceExpr().TAco("won't").TWs().TAco("allowed").TWs().TAco("to").TWs().ThenVerb(), 2)
	return &BeAllowed{e}
}
func (r *BeAllowed) Expr() expr.Expr                  { return r.expression }
func (r *BeAllowed) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *BeAllowed) Description() string {
	return "Ensures the passive form uses `be allowed` after future negatives."
}
func (r *BeAllowed) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	i, ok := r.expression.Lookup(0, toks, src)
	if !ok || i >= len(toks) {
		return nil
	}
	result := lint.NewLint()
	result.Span = toks[i].Span
	result.LintKind = lint.Grammar
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("be allowed", toks[i].GetCh(src))}
	result.Message = "Add `be` so this reads `be allowed`."
	result.Priority = 31
	return &result
}
