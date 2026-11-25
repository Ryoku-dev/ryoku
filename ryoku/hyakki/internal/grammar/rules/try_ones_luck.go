package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type TryOnesLuck struct{ expression expr.Expr }

func NewTryOnesLuck() *TryOnesLuck {
	return &TryOnesLuck{expression: expr.NewSequenceExprWordSet([]string{"try", "tried", "tries", "trying"}).TWs().TAco("out").TWs().ThenPossessiveDeterminer().TWs().TAco("luck")}
}
func (r *TryOnesLuck) Expr() expr.Expr                  { return r.expression }
func (r *TryOnesLuck) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *TryOnesLuck) Description() string {
	return "Corrects `try out one’s luck` to `try one’s luck`"
}
func (r *TryOnesLuck) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 3 {
		return nil
	}
	span, ok := doc.Span(toks[1:3])
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.Remove()}, Message: "`Try out` is a different idiom than `try one's luck`", Priority: 127}
}
