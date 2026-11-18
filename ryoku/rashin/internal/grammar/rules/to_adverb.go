package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/pos"
	"ryoku-rashin/internal/grammar/text"
)

type ToAdverb struct{ expression expr.Expr }

func NewToAdverb() *ToAdverb {
	return &ToAdverb{expression: expr.NewSequenceExprAco("to").TWs().Then(expr.Or(expr.NewUPOSSet([]pos.UPOS{pos.ADV}), expr.NewWordSet([]string{"just", "not"}))).TWs().TAco("to").TWs().ThenVerb()}
}
func (r *ToAdverb) Expr() expr.Expr                  { return r.expression }
func (r *ToAdverb) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*ToAdverb) Description() string {
	return "Flags duplicated `to` around certain adverbs (e.g. `to never to`) and offers fixes that keep only one `to`."
}
func (r *ToAdverb) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 5 {
		return nil
	}
	first, adverb, second := toks[0], toks[2], toks[4]
	keep := src[first.Span.Start:adverb.Span.End]
	drop := src[adverb.Span.Start:second.Span.End]
	if len(keep) == 0 || len(drop) == 0 {
		return nil
	}
	return &lint.Lint{Span: text.Span{Start: first.Span.Start, End: second.Span.End}, LintKind: lint.Miscellaneous, Suggestions: []lint.Suggestion{lint.ReplaceWith(append([]rune(nil), keep...)), lint.ReplaceWith(append([]rune(nil), drop...))}, Message: "Remove the repeated `to` in this infinitive.", Priority: 40}
}
