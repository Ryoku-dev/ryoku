package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type AskNoPreposition struct{ expression expr.Expr }

func NewAskNoPreposition() *AskNoPreposition {
	return &AskNoPreposition{expression: expr.NewSequenceExprWith(expr.NewWordSet([]string{"ask", "asks", "asked", "asking", "tell", "tells", "told", "telling"})).ThenWhitespace().TAco("to").ThenWhitespace().Then(expr.NewWordSet([]string{"me", "you", "him", "her", "it", "us", "them", "one"}))}
}
func (r *AskNoPreposition) Expr() expr.Expr                  { return r.expression }
func (r *AskNoPreposition) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *AskNoPreposition) Description() string {
	return "Identifies sequences like `ask to us` or `tell to him` and recommends removing the superfluous “to”."
}
func (r *AskNoPreposition) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 5 {
		return nil
	}
	result := lint.NewLint()
	result.Span = text.Span{Start: toks[2].Span.Start, End: toks[3].Span.End}
	result.LintKind = lint.WordChoice
	result.Suggestions = []lint.Suggestion{lint.ReplaceWith(nil)}
	result.Message = "The verb `to " + string(text.ToLower(toks[0].GetCh(src))) + " someone` should not be preceded by the preposition `to`."
	result.Priority = 63
	return &result
}
