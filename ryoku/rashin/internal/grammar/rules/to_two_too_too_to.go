package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/pos"
)

type TooTo struct{ expression expr.Expr }

func NewTooTo() *TooTo {
	return &TooTo{expression: expr.NewSequenceExprAco("too").TWs().Then(expr.NewUPOSSet([]pos.UPOS{pos.NOUN, pos.PRON, pos.PROPN, pos.VERB, pos.DET}))}
}
func (r *TooTo) Expr() expr.Expr                  { return r.expression }
func (r *TooTo) Description() string              { return "Handles the transition from `too` -> `to`." }
func (r *TooTo) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *TooTo) MatchToLint(tokens []doc.Token, source []rune) *lint.Lint {
	if len(tokens) > 2 && tokens[2].Kind.IsUpos(pos.VERB) && !tokens[2].Kind.IsVerbLemma() {
		return nil
	}
	if len(tokens) <= 0 {
		return nil
	}
	target := tokens[0]
	result := lint.NewLint()
	result.Span = target.Span
	result.LintKind = lint.WordChoice
	result.Priority = 31
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("to", target.GetCh(source))}
	result.Message = "Use the infinitive marker `to` here instead of the adverb `too`, which indicates excess degree."
	return &result
}
