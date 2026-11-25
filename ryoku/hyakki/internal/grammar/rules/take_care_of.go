package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type TakeCareOf struct{ expression expr.Expr }

func NewTakeCareOf() *TakeCareOf {
	return &TakeCareOf{expression: expr.NewSequenceExprWordSet([]string{"take", "taken", "takes", "taking", "took"}).TWs().ThenWordSeq([]string{"care", "about"})}
}
func (r *TakeCareOf) Expr() expr.Expr                  { return r.expression }
func (r *TakeCareOf) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *TakeCareOf) Description() string              { return "Corrects `take care about` to `take care of`." }
func (r *TakeCareOf) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	tok := t[len(t)-1]
	result := lint.NewLint()
	result.Span = tok.Span
	result.LintKind = lint.Usage
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("of", tok.GetCh(s))}
	result.Message = "Are you confusing `care about` with `take care of`?"
	return &result
}
