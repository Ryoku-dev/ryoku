package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type FallBelow struct{ expression expr.Expr }

func NewFallBelow() *FallBelow {
	return &FallBelow{expression: expr.NewSequenceExprWordSet([]string{"fall", "fallen", "falling", "falls", "fell"}).TWs().ThenAnyOf([]expr.Expr{expr.NewSequenceExprNumber(), expr.NewSpelledNumberExpr()}).TWs().TSet([]string{"feet", "meters", "metres"}).TWs().TAco("below").ThenAnyOf([]expr.Expr{expr.NewAnchorEnd(), expr.NewSequenceExprWhitespace().TAco("and")})}
}
func (r *FallBelow) Expr() expr.Expr { return r.expression }
func (r *FallBelow) Description() string {
	return "Flags redundant usage of `below` after fall distances."
}
func (r *FallBelow) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *FallBelow) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Usage

	tok := lint.FindTheOnlyTokenMatching(toks, src, func(t *doc.Token, s []rune) bool { return strings.EqualFold(t.GetStr(s), "below") })
	if tok == nil {
		return nil
	}
	result.Span = tok.Span
	if result.Span.Start > 0 {
		result.Span.Start--
	}
	result.Suggestions = []lint.Suggestion{lint.Remove()}
	result.Message = "The word below is unnecessary here. Consider dropping it or specifying what the subject fell below."
	return &result
}
