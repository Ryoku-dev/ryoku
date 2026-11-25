package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type TheTheToThatThe struct{ expression expr.Expr }

func NewTheTheToThatThe() *TheTheToThatThe {
	return &TheTheToThatThe{expression: expr.NewSequenceExprWordSet([]string{"so", "fact", "found", "show", "say", "believe", "think", "said"}).TWs().TAco("the").TWs().TAco("the")}
}
func (r *TheTheToThatThe) Expr() expr.Expr { return r.expression }
func (r *TheTheToThatThe) Description() string {
	return "Corrects `the the` to `that the` or to a single `the`."
}
func (r *TheTheToThatThe) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *TheTheToThatThe) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	for i, tok := range t {
		if text.EqStr(tok.GetCh(s), "the") && i+2 < len(t) {
			span := text.Span{Start: tok.Span.Start, End: t[i+2].Span.End}
			result := lint.NewLint()
			result.Span = span
			result.LintKind = lint.Typo
			result.Suggestions = lint.ToReplaceSuggestions([]string{"that the", "the"}, span.GetContent(s))
			result.Message = "Did you mean `that the` or just `the`?"
			result.Priority = 126
			return &result
		}
	}
	return nil
}
