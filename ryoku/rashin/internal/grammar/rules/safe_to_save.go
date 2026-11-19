package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/pos"
	"strings"
)

type SafeToSave struct{ expression expr.Expr }

func NewSafeToSave() *SafeToSave {
	return &SafeToSave{expression: expr.OrLongest(expr.NewSequenceExprWith(expr.NewModalVerb()).ThenWhitespace().Then(expr.NewUPOSSet([]pos.UPOS{pos.ADV})).ThenWhitespace().TAco("safe").ThenWhitespace().ThenUnless(expr.NewWordSet([]string{"to"})), expr.NewSequenceExprWith(expr.NewModalVerb()).ThenWhitespace().TAco("safe").ThenWhitespace().ThenUnless(expr.NewWordSet([]string{"to"})))}
}
func (r *SafeToSave) Expr() expr.Expr                  { return r.expression }
func (r *SafeToSave) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*SafeToSave) Description() string {
	return "Detects `safe` (adjective) when `save` (verb) is intended after modal verbs like `could` or `should`."
}
func (r *SafeToSave) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	for _, tok := range toks {
		if strings.ToLower(tok.GetStr(src)) == "safe" {
			return &lint.Lint{Span: tok.Span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWith([]rune("save"))}, Message: "The word `safe` is an adjective. Did you mean the verb `save`?", Priority: 57}
		}
	}
	return nil
}
