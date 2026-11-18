package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type OutOfCuriosity struct{ expression expr.Expr }

func NewOutOfCuriosity() *OutOfCuriosity {
	return &OutOfCuriosity{expression: expr.NewSequenceExprWordSeq([]string{"out", "of", "curiosities"})}
}
func (r *OutOfCuriosity) Expr() expr.Expr                  { return r.expression }
func (r *OutOfCuriosity) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*OutOfCuriosity) Description() string {
	return "A linter skeleton for contributors to copy into `harper_core/src/linting/` and rename."
}
func (r *OutOfCuriosity) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}

func (*OutOfCuriosity) Unit() lint.DocumentIterator { return lint.Sentence{} }
func (*OutOfCuriosity) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	if len(toks) < 5 {
		return nil
	}
	target, message := "", ""
	if lint.FollowedByToken(ctx, func(t *doc.Token) bool { return t.Kind.IsComma() }) {
		target = "curiosity"
		message = "This idiom uses the singular `curiosity`."
	} else if lint.FollowedByWord(ctx, func(t *doc.Token) bool { return text.EqStr(t.GetCh(src), "sake") }) {
		target = "curiosity's"
		message = "It looks like you mean the possessive `curiosity's`."
	} else {
		return nil
	}
	result := lint.NewLint()
	result.Span = toks[4].Span
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(target, result.Span.GetContent(src))}
	result.Message = message
	return &result
}
