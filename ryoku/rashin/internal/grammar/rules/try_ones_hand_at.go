package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type TryOnesHandAt struct{ expression expr.Expr }

func NewTryOnesHandAt() *TryOnesHandAt {
	return &TryOnesHandAt{expression: expr.NewSequenceExprWordSet([]string{"try", "tried", "tries", "trying"}).TWs().ThenPossessiveDeterminer().TWs().TAco("hands").TWs().TAco("at")}
}
func (r *TryOnesHandAt) Expr() expr.Expr                  { return r.expression }
func (r *TryOnesHandAt) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*TryOnesHandAt) Description() string {
	return "Corrects `try one's hands at` to `try one's hand at`."
}
func (r *TryOnesHandAt) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	tok := t[4]
	l := lint.NewLint()
	l.Span = tok.Span
	l.LintKind = lint.Usage
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("hand", tok.GetCh(s))}
	l.Message = "This idiom uses the singular `hand`."
	return &l
}
