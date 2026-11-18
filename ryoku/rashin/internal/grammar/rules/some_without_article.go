package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type SomeWithoutArticle struct{ expression expr.Expr }

func NewSomeWithoutArticle() *SomeWithoutArticle {
	return &SomeWithoutArticle{expression: expr.NewSequenceExprWordSeq([]string{"the", "some"})}
}
func (r *SomeWithoutArticle) Expr() expr.Expr                  { return r.expression }
func (r *SomeWithoutArticle) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*SomeWithoutArticle) Description() string {
	return "Detects the redundant article in front of `some` and suggests more natural phrasing."
}
func (r *SomeWithoutArticle) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.WordChoice
	l.Message = "Use `some` on its own here, or switch to `the same` if that was the intention."
	l.Suggestions = []lint.Suggestion{lint.ReplaceWith(append([]rune(nil), toks[len(toks)-1].GetCh(src)...)), lint.ReplaceWithMatchCaseStr("the same", span.GetContent(src))}
	return &l
}
