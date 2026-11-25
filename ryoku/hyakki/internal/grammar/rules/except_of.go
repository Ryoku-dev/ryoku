package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ExceptOf struct{ expression expr.Expr }

func NewExceptOf() *ExceptOf {
	return &ExceptOf{expression: expr.NewFixedPhraseFromPhrase("except of")}
}
func (r *ExceptOf) Expr() expr.Expr                  { return r.expression }
func (r *ExceptOf) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ExceptOf) Description() string {
	return "Corrects `except of` to `except for` or `exception of`."
}
func (r *ExceptOf) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return r.MatchToLintWithContext(t, s, nil)
}
func (r *ExceptOf) MatchToLintWithContext(t []doc.Token, s []rune, c *lint.MatchContext) *lint.Lint {
	if lint.FollowedByWord(c, func(t *doc.Token) bool { return text.EqStr(t.GetCh(s), "course") }) {
		return nil
	}
	span := t[2].Span
	replacement := "for"
	message := "Use `except for` instead of `except of`"
	if lint.PrecededByWord(c, func(t *doc.Token) bool {
		return text.EqAnyIgnoreASCIICaseStr(t.GetCh(s), []string{"the", "notable", "possible"})
	}) {
		span = t[0].Span
		replacement = "exception"
		message = "This usage requires the noun 'exception'. Use 'exception of' instead of 'except of'"
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Usage
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(replacement, s[span.Start:span.End])}
	result.Message = message
	return &result
}
