package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type SubjectPronoun struct{ expression expr.Expr }

func NewSubjectPronoun() *SubjectPronoun {
	return &SubjectPronoun{expression: expr.NewSequenceExprWith(expr.NewAnchorStart()).TAco("me").TWs().TAco("and").TWs().ThenProperNoun()}
}
func (r *SubjectPronoun) Expr() expr.Expr                  { return r.expression }
func (r *SubjectPronoun) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *SubjectPronoun) Description() string {
	return "Fixes sentences that start with `me and X` by putting the proper noun first and using `I`."
}
func (r *SubjectPronoun) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	replacement := append([]rune(nil), toks[len(toks)-1].GetCh(src)...)
	replacement = append(replacement, []rune(" and I")...)
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Grammar
	result.Suggestions = []lint.Suggestion{lint.ReplaceWith(replacement)}
	result.Message = "Put the other person first and use `I` in this compound subject."
	result.Priority = 31
	return &result
}
