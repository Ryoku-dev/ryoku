package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/pos"
)

type FindFine struct{ expression expr.Expr }

func NewFindFine() *FindFine {
	return &FindFine{expression: expr.NewSequenceExprWith(expr.NewInflectionOfBe()).TWs().TAco("find").ThenUnless(expr.NewSequenceExprWhitespace().Then(expr.NewUPOSSet([]pos.UPOS{pos.NOUN, pos.PROPN, pos.PRON, pos.DET})))}
}
func (r *FindFine) Expr() expr.Expr                  { return r.expression }
func (r *FindFine) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *FindFine) Description() string {
	return "Fixes the common typo where writers write `find` when they mean `fine`."
}
func (r *FindFine) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 3 {
		return nil
	}
	result := lint.NewLint()
	result.Span = toks[2].Span
	result.LintKind = lint.Typo
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("fine", toks[2].GetCh(src))}
	result.Message = "Did you mean `fine`?"
	result.Priority = 63
	return &result
}
