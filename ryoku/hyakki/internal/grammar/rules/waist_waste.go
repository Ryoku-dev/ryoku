package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type WaistWaste struct{ expression expr.Expr }

func NewWaistWaste() *WaistWaste {
	return &WaistWaste{expression: expr.NewFirstMatchOf([]expr.Expr{expr.NewSequenceExprAnyWord().TWs().Then(expr.NewInflectionOfBe()).TWs().ThenWordSeq([]string{"a", "waist", "of"}), expr.NewSequenceExprWordSeq([]string{"waist", "of"}).TWs().TSet([]string{"effort", "money", "resources", "space", "time"}), expr.NewSequenceExprWordSet([]string{"complete", "confusing", "great", "horrible", "huge", "real", "total", "utter"}).TWs().ThenWordSeq([]string{"waist", "of"}), expr.NewSequenceExprWordSeq([]string{"a", "waist", "of"}).TWs().ThenPossessiveDeterminer(), expr.NewSequenceExprWordSet([]string{"definitely", "just", "mostly", "probably", "quite", "really"}).TWs().ThenWordSeq([]string{"a", "waist", "of"})})}
}
func (r *WaistWaste) Expr() expr.Expr { return r.expression }
func (r *WaistWaste) Description() string {
	return "Corrects misspelling `waste` (careless use) as `waist` (body part)."
}
func (r *WaistWaste) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *WaistWaste) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	tok := lint.FindTheOnlyTokenMatching(t, s, func(w *doc.Token, s []rune) bool { return text.EqStr(w.GetCh(s), "waist") })
	if tok == nil {
		return nil
	}
	result := lint.NewLint()
	result.Span = tok.Span
	result.LintKind = lint.WordChoice
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("waste", tok.GetCh(s))}
	result.Message = "Did you mean `waste` (careless use) rather than `waist` (body part)?"
	return &result
}
