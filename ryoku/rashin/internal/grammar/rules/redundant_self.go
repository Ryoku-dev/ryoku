package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type RedundantSelf struct{ expression expr.Expr }

func NewRedundantSelf() *RedundantSelf {
	return &RedundantSelf{expression: expr.NewFirstMatchOf([]expr.Expr{expr.ButNot(expr.NewSequenceExprAco("self").TWsH().TSet([]string{"censor", "censored", "censors", "censoring", "correct", "corrected", "correcting", "corrects", "defend", "defended", "defending", "defends", "describe", "described", "describes", "describing", "diagnose", "diagnosed", "diagnoses", "diagnosing", "govern", "governed", "governs", "governing", "harm", "harmed", "harming", "harms", "identify", "identified", "identifies", "identifying", "level", "leveled", "leveling", "levelled", "levelling", "levels", "medicate", "medicated", "medicates", "medicating", "regulate", "regulated", "regulates", "regulating", "right", "righted", "righting", "rights", "sabotage", "sabotaged", "sabotages", "sabotaging", "taught", "teach", "teaches", "teaching"}).TWs().Then(expr.NewReflexivePronounWithCommonErrors()), expr.NewSequenceExprAnything().TAny().TAco("harm").TAny().TAco("itself")), expr.NewSequenceExprAco("self").TWsH().TAco("harm").TWs().TAco("to").TWs().Then(expr.NewReflexivePronounWithCommonErrors()), expr.NewSequenceExprAco("self").TWsH().TSet([]string{"host", "hosted", "hosting", "hosts"}).TWs().TSet([]string{"it", "them", "one"}).TWs().Then(expr.NewReflexivePronounWithCommonErrors())})}
}
func (r *RedundantSelf) Expr() expr.Expr { return r.expression }
func (r *RedundantSelf) Description() string {
	return "Detects redundant use of `self-` prefixes with reflexive pronouns (e.g., `self-host it themselves`)."
}
func (r *RedundantSelf) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *RedundantSelf) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Redundancy

	var ranges [][2]int
	switch {
	case len(toks) == 5:
		ranges = [][2]int{{0, 3}, {2, 5}}
	case len(toks) == 7 && strings.EqualFold(toks[4].GetStr(src), "to"):
		ranges = [][2]int{{0, 3}, {2, 7}}
	case len(toks) == 7:
		ranges = [][2]int{{0, 5}, {2, 7}}
	default:
		return nil
	}
	for _, r := range ranges {
		replacement := []rune{}
		for _, tok := range toks[r[0]:r[1]] {
			replacement = append(replacement, tok.GetCh(src)...)
		}
		result.Suggestions = append(result.Suggestions, lint.ReplaceWithMatchCase(replacement, src[span.Start:span.End]))
	}
	result.Message = "Avoid redundancy by using `self` only once."
	return &result
}
