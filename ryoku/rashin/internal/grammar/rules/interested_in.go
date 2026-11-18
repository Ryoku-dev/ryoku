package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type InterestedIn struct{ expression expr.Expr }

func NewInterestedIn() *InterestedIn {
	return &InterestedIn{expression: expr.NewSequenceExprAco("interested").TWs().ThenKindExcept(doc.TokenKind.IsPreposition, []string{"around", "for", "through", "to", "within"})}
}
func (r *InterestedIn) Expr() expr.Expr { return r.expression }
func (r *InterestedIn) Description() string {
	return "Ensures the correct preposition is used with the word `interested` (e.g. `interested in`)."
}
func (r *InterestedIn) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *InterestedIn) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	span := toks[len(toks)-1].Span
	if text.EqCh(src[span.Start:span.End], []rune("in")) {
		return nil
	}
	suggestions := []lint.Suggestion{lint.ReplaceWithMatchCaseStr("in", src[span.Start:span.End])}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Usage
	result.Message = "The correct preposition to use with `interested` is `in`."
	result.Suggestions = suggestions
	return &result
}
