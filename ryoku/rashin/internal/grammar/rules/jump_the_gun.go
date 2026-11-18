package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type JumpTheGun struct{ expression expr.Expr }

func NewJumpTheGun() *JumpTheGun {
	return &JumpTheGun{expression: expr.ButNot(expr.NewSequenceExprWordSet([]string{"jump", "jumped", "jumping", "jumps"}).TWs().TSet([]string{"a", "an", "the"}).TWs().TSet([]string{"gun", "guns"}), expr.NewSequenceExprAnything().TAny().TAco("the").TWs().TAco("gun"))}
}
func (r *JumpTheGun) Expr() expr.Expr                  { return r.expression }
func (r *JumpTheGun) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*JumpTheGun) Description() string {
	return "Detects incorrect usage of the `jump the gun` idiom."
}
func (r *JumpTheGun) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 3 {
		return nil
	}
	span, ok := doc.Span(toks[2:])
	if !ok {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Usage
	l.Message = "The correct idiom is `jump the gun`"
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("the gun", span.GetContent(src))}
	return &l
}
