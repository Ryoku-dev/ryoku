package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type AndTheLike struct{ expression expr.Expr }

func NewAndTheLike() *AndTheLike {
	return &AndTheLike{expression: expr.NewAll([]expr.Expr{expr.NewSequenceExprWordSet([]string{"and", "or", "an"}).TWs().ThenOptional(expr.NewSequenceExprAco("the").TWs()).ThenWordSet([]string{"alike", "alikes", "like", "likes"}), expr.NewSequenceExprUnless(expr.NewSequenceExprWordSet([]string{"and", "or"}).TWs().ThenAnyOf([]expr.Expr{expr.NewFixedPhraseFromPhrase("the like"), expr.AsExpr(expr.NewWordSet([]string{"like", "likes"}))}))})}
}
func (r *AndTheLike) Expr() expr.Expr                  { return r.expression }
func (r *AndTheLike) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *AndTheLike) Description() string {
	return "Corrects mistakes in `and the like` and `or the like`."
}
func (r *AndTheLike) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	conj := "or"
	if toks[0].GetCh(src)[0] == 'a' {
		conj = "and"
	}
	corrected := conj + toks[1].GetStr(src) + "the like"
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Usage
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(corrected, span.GetContent(src))}
	result.Message = "If you intended the idiom meaning `similar things`, the correct form is with `the like`."
	return &result
}
