package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type HopeYoure struct{ expression expr.Expr }

func NewHopeYoure() *HopeYoure {
	return &HopeYoure{expression: expr.NewSequenceExprWordSeq([]string{"hope", "your"}).TWs().ThenAdjective().ThenOptional(expr.NewSequenceExprWhitespace().ThenPreposition()).TWs().Then(expr.NewWordSet([]string{"here", "there"}))}
}
func (r *HopeYoure) Expr() expr.Expr                  { return r.expression }
func (r *HopeYoure) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *HopeYoure) Description() string {
	return "Detects the misuse of possessive **your** after “hope” and an adjective (e.g., “I hope your well”) and advises using **you’re** to supply the missing verb “are.”"
}
func (r *HopeYoure) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 3 {
		return nil
	}
	tok := toks[2]
	l := lint.NewLint()
	l.Span = tok.Span
	l.LintKind = lint.WordChoice
	l.Priority = 31
	l.Message = "Prefer `you're`—the contraction of “you are”—when expressing a hope about someone’s condition."
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("you're", tok.GetCh(src))}
	return &l
}
