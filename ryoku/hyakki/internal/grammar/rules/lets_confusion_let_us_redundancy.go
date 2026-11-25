package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type LetUsRedundancy struct{ expression expr.Expr }

func NewLetUsRedundancy() *LetUsRedundancy {
	return &LetUsRedundancy{expression: expr.NewSequenceExprAco("let's").TWs().ThenObjectPronoun()}
}
func (r *LetUsRedundancy) Expr() expr.Expr { return r.expression }
func (r *LetUsRedundancy) Description() string {
	return "Many are not aware that the contraction `let's` is short for `let us`. As a result, many will incorrectly use it before a pronoun, such as in the phrase `let's us do`."
}
func (r *LetUsRedundancy) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *LetUsRedundancy) MatchToLint(tokens []doc.Token, source []rune) *lint.Lint {
	span, ok := doc.Span(tokens)
	if !ok {
		return nil
	}
	pronoun := tokens[len(tokens)-1].GetStr(source)
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Repetition
	result.Priority = 31
	result.Message = "`let's` stands for `let us`, so including another pronoun is redundant."
	result.Suggestions = lint.ToReplaceSuggestions([]string{"lets " + pronoun, "let's"}, source[span.Start:span.End])
	return &result
}
