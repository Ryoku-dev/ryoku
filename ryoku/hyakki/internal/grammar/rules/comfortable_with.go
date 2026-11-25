package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ComfortableWith struct{ expression expr.Expr }

func NewComfortableWith() *ComfortableWith {
	return &ComfortableWith{expression: expr.ButNot(expr.NewSequenceExprWordSeq([]string{"comfortable", "of"}), expr.NewSequenceExprAnything().TAny().TAny().TWs().TSet([]string{"all", "course", "those"}))}
}
func (r *ComfortableWith) Expr() expr.Expr                  { return r.expression }
func (r *ComfortableWith) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ComfortableWith) Description() string {
	return "Corrects `comfortable of` to `comfortable with`."
}
func (r *ComfortableWith) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
func (r *ComfortableWith) MatchToLintWithContext(toks []doc.Token, src []rune, context *lint.MatchContext) *lint.Lint {
	if lint.PrecededByWord(context, func(t *doc.Token) bool { return text.EqStr(t.GetCh(src), "most") }) {
		return nil
	}
	if len(toks) < 3 {
		return nil
	}
	result := lint.NewLint()
	result.Span = toks[2].Span
	result.LintKind = lint.Usage
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("with", toks[2].GetCh(src))}
	result.Message = "The standard preposition after `comfortable` is `with`"
	return &result
}
