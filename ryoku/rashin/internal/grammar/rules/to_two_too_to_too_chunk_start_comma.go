package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ToTooChunkStartComma struct{ expression expr.Expr }

func NewToTooChunkStartComma() *ToTooChunkStartComma {
	return &ToTooChunkStartComma{expression: expr.NewSequenceExprWith(expr.NewAnchorStart()).TAco("to").ThenOptionalWhitespace().ThenComma()}
}
func (r *ToTooChunkStartComma) Expr() expr.Expr { return r.expression }
func (r *ToTooChunkStartComma) Description() string {
	return "Detects `to` at the start of a clause before a comma."
}
func (r *ToTooChunkStartComma) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ToTooChunkStartComma) MatchToLint(tokens []doc.Token, source []rune) *lint.Lint {
	var target *doc.Token
	for i := range tokens {
		if text.EqStr(tokens[i].GetCh(source), "to") {
			target = &tokens[i]
			break
		}
	}
	if target == nil {
		return nil
	}
	result := lint.NewLint()
	result.Span = target.Span
	result.LintKind = lint.WordChoice
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("too", target.GetCh(source))}
	result.Message = "Use `too` here to mean ‘also’ or an excessive degree."
	return &result
}
