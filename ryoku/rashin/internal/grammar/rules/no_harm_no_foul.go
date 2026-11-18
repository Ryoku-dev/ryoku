package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type NoHarmNoFoul struct{ expression expr.Expr }

func NewNoHarmNoFoul() *NoHarmNoFoul {
	e := expr.ButNot(expr.NewSequenceExprWordSet([]string{"no", "nor"}).TWs().TAco("harm").ThenOptionalComma().TWs().TSet([]string{"no", "nor"}).TWs().TSet([]string{"fowl", "foul"}), expr.NewSequenceExprWordSeq([]string{"no", "harm"}).ThenOptionalComma().TWs().ThenWordSeq([]string{"no", "foul"}))
	return &NoHarmNoFoul{expression: e}
}
func (r *NoHarmNoFoul) Expr() expr.Expr { return r.expression }
func (r *NoHarmNoFoul) Description() string {
	return "Corrects nonstandard variants of the idiom `no harm, no foul`."
}
func (r *NoHarmNoFoul) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *NoHarmNoFoul) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	mid, _ := doc.GetStr(t[3:len(t)-3], s)
	correction := "no harm" + mid + "no foul"
	usesNor := text.EndsWithIgnoreASCIICaseStr(t[0].GetCh(s), "r") || text.EndsWithIgnoreASCIICaseStr(t[len(t)-3].GetCh(s), "r")
	fowl := text.EqStr(t[len(t)-1].GetCh(s), "fowl")
	message := ""
	if usesNor && fowl {
		message = "Use the words `no` and `foul` in this idiom."
	} else if usesNor {
		message = "This idiom uses the word `no` rather than `not`."
	} else if fowl {
		message = "`Fowl` means a bird. This idiom uses `foul`, as in baseball."
	} else {
		return nil
	}
	span, _ := doc.Span(t)
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Nonstandard
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(correction, span.GetContent(s))}
	l.Message = message
	return &l
}
func (*NoHarmNoFoul) Unit() lint.DocumentIterator { return lint.Sentence{} }
