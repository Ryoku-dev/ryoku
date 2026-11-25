package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type Dashes struct{ expression expr.Expr }

func NewDashes() *Dashes {
	return &Dashes{expression: expr.NewLongestMatchOf([]expr.Expr{expr.NewSequenceExpr().ThenHyphen().ThenHyphen().ThenOneOrMoreHyphens(), expr.NewSequenceExpr().ThenHyphen().ThenHyphen()})}
}
func (r *Dashes) Expr() expr.Expr { return r.expression }
func (r *Dashes) Description() string {
	return "Writers often type `--` or `---` expecting their editor to convert them into proper dashes. Replace these sequences with the correct characters: use an en dash (–) for ranges or connections and an em dash (—) for a break in thought."
}
func (r *Dashes) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *Dashes) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) >= 4 {
		return nil
	}
	if len(t) != 2 && len(t) != 3 {
		panic("Received unexpected number of tokens.")
	}
	result := lint.NewLint()
	result.Span = text.Span{Start: t[0].Span.Start, End: t[len(t)-1].Span.End}
	result.LintKind = lint.Formatting
	result.Priority = 63
	if len(t) == 2 {
		result.Suggestions = []lint.Suggestion{lint.ReplaceWith([]rune("–"))}
		result.Message = "Replace these two hyphens with an en dash (–)."
	} else {
		result.Suggestions = []lint.Suggestion{lint.ReplaceWith([]rune("—"))}
		result.Message = "Replace these three hyphens with an em dash (—)."
	}
	return &result
}
