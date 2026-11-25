package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ThrowBabyWithBathwater struct{ expression expr.Expr }

func NewThrowBabyWithBathwater() *ThrowBabyWithBathwater {
	baby := expr.NewSequenceExprWordSeq([]string{"the", "baby"}).TWs()
	away := expr.NewSequenceExprWordSeq([]string{"away"}).TWs()
	return &ThrowBabyWithBathwater{expression: expr.NewSequenceExprWordSet([]string{"throw", "threw", "thrown", "throwing", "throws", "through"}).TWs().ThenAnyOf([]expr.Expr{baby, expr.NewSequenceExprWith(away).Then(baby), expr.NewSequenceExprWith(baby).Then(away)}).ThenWordSeq([]string{"with", "the"}).TWs().ThenAnyOf([]expr.Expr{expr.AsExpr(expr.NewWord("bathwater")), expr.NewSequenceExprWordSeq([]string{"bath", "water"})})}
}
func (r *ThrowBabyWithBathwater) Expr() expr.Expr { return r.expression }
func (r *ThrowBabyWithBathwater) Description() string {
	return "Corrects wrong or nonstandard variants of the idiom 'to throw the baby out with the bathwater'"
}
func (r *ThrowBabyWithBathwater) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ThrowBabyWithBathwater) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) < 7 {
		return nil
	}
	for _, i := range []int{2, 6} {
		if text.EqStr(t[i].GetCh(s), "away") {
			result := lint.NewLint()
			result.Span = t[i].Span
			result.LintKind = lint.Nonstandard
			result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("out", t[i].GetCh(s))}
			result.Message = "This idiom uses `throw out` rather than `throw away`."
			return &result
		}
	}
	span := text.Span{Start: t[2].Span.Start, End: t[4].Span.End}
	chars := span.GetContent(s)
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Usage
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("out "+string(chars), chars), lint.ReplaceWithMatchCaseStr(string(chars)+" out", chars)}
	result.Message = "This idiom requires `throw out` rathern than just `throw`."
	return &result
}
