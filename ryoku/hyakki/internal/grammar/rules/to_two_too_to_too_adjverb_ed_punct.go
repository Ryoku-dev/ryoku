package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ToTooAdjVerbEdPunct struct{ expression expr.Expr }

func NewToTooAdjVerbEdPunct() *ToTooAdjVerbEdPunct {
	return &ToTooAdjVerbEdPunct{expression: expr.NewSequenceExprAco("to").TWs().Then(func(tok doc.Token, src []rune) bool {
		return tok.Kind.IsAdjective() && tok.Kind.IsVerb() && !tok.Kind.IsNoun() && text.EndsWithIgnoreASCIICaseStr(tok.GetCh(src), "ed")
	}).ThenSentenceTerminator()}
}
func (r *ToTooAdjVerbEdPunct) Expr() expr.Expr { return r.expression }
func (r *ToTooAdjVerbEdPunct) Description() string {
	return "Detects `to` before words that are adj/verb ending with `ed`, followed by punctuation."
}
func (r *ToTooAdjVerbEdPunct) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ToTooAdjVerbEdPunct) MatchToLint(tokens []doc.Token, source []rune) *lint.Lint {
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
