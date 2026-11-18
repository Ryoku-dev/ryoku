package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"unicode"
)

type SinceDuration struct{ expression expr.Expr }

func NewSinceDuration() *SinceDuration {
	return &SinceDuration{expression: expr.NewSequenceExprAnyCapitalizationOf("since").ThenWhitespace().Then(expr.NewDurationExpr()).ThenOptional(expr.NewSequenceExpr().TWs().ThenWordSet([]string{"ago", "old"}))}
}
func (r *SinceDuration) Expr() expr.Expr                  { return r.expression }
func (r *SinceDuration) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *SinceDuration) Description() string {
	return "Detects the use of 'since' with a duration instead of a point in time."
}
func (r *SinceDuration) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	last := toks[len(toks)-1]
	if text.EqAnyIgnoreASCIICaseStr(last.GetCh(src), []string{"ago", "old"}) {
		return nil
	}
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	ago := append([]rune(nil), span.GetContent(src)...)
	ago = append(ago, ' ')
	ago = append(ago, []rune(sinceDurationMatchCaseString(last.GetCh(src), [3]string{"ago", "Ago", "AGO"}))...)
	duration, ok := doc.Span(toks[1:])
	if !ok {
		return nil
	}
	forDuration := append([]rune(sinceDurationMatchCaseString(toks[0].GetCh(src), [3]string{"for", "For", "FOR"})), duration.GetContent(src)...)
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Miscellaneous
	result.Suggestions = []lint.Suggestion{lint.ReplaceWith(forDuration), lint.ReplaceWith(ago)}
	result.Message = "For a duration, use 'for' instead of 'since'. Or for a point in time, add 'ago' at the end."
	result.Priority = 50
	return &result
}
func sinceDurationMatchCaseString(template []rune, variants [3]string) string {
	c1 := template[0]
	c2 := ' '
	if len(template) > 1 {
		c2 = template[1]
	}
	if unicode.IsUpper(c1) && unicode.IsUpper(c2) {
		return variants[2]
	}
	if unicode.IsUpper(c1) {
		return variants[1]
	}
	return variants[0]
}
