package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type RedundantAlmostNearly struct{ expression expr.Expr }

func NewRedundantAlmostNearly() *RedundantAlmostNearly {
	return &RedundantAlmostNearly{expression: expr.NewFirstMatchOf([]expr.Expr{expr.NewSequenceExprWordSeq([]string{"almost", "nearly"}), expr.NewSequenceExprWordSeq([]string{"nearly", "almost"})})}
}
func (r *RedundantAlmostNearly) Expr() expr.Expr                  { return r.expression }
func (r *RedundantAlmostNearly) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*RedundantAlmostNearly) Description() string {
	return "Detects `almost` and `nearly` redundantly used together."
}
func (r *RedundantAlmostNearly) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Redundancy, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("almost", span.GetContent(src)), lint.ReplaceWithMatchCaseStr("nearly", span.GetContent(src))}, Message: "Use just one of `almost` or `nearly`.", Priority: 31}
}
