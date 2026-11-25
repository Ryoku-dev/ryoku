package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/weir"
)

type MapPhraseLinter struct {
	description  string
	expression   expr.Expr
	correctForms []string
	message      string
	lintKind     lint.LintKind
}

func NewMapPhraseLinter(expression expr.Expr, correctForms []string, message, description string, kind *lint.LintKind) *MapPhraseLinter {
	k := lint.Miscellaneous
	if kind != nil {
		k = *kind
	}
	return &MapPhraseLinter{description: description, expression: expression, correctForms: append([]string(nil), correctForms...), message: message, lintKind: k}
}
func NewMapPhraseLinterSimilarToPhrase(phrase string, detectableDistance uint8) *MapPhraseLinter {
	return NewMapPhraseLinter(expr.NewSimilarToPhraseFromPhrase(phrase, detectableDistance), []string{phrase}, fmt.Sprintf("Did you mean the phrase `%s`?", phrase), fmt.Sprintf("Looks for slight improper modifications to the phrase `%s`.", phrase), nil)
}
func NewMapPhraseLinterFixedPhrases(phrases, correctForms []string, message, description string, kind *lint.LintKind) *MapPhraseLinter {
	patterns := expr.NewLongestMatchOf()
	for _, p := range phrases {
		e, err := weir.WeirExprToExpr(p)
		if err != nil {
			panic(err)
		}
		patterns.Add(e)
	}
	return NewMapPhraseLinter(patterns, correctForms, message, description, kind)
}
func NewMapPhraseLinterFixedPhrase(phrase string, correctForms []string, message, description string, kind *lint.LintKind) *MapPhraseLinter {
	return NewMapPhraseLinter(expr.NewFixedPhraseFromPhrase(phrase), correctForms, message, description, kind)
}
func NewMapPhraseLinterClosedCompound(phrases []string, correctForm string) *MapPhraseLinter {
	kind := lint.Miscellaneous
	return NewMapPhraseLinterFixedPhrases(phrases, []string{correctForm}, fmt.Sprintf("Did you mean the closed compound `%s`?", correctForm), fmt.Sprintf("Looks for incorrect spacing inside the closed compound `%s`.", correctForm), &kind)
}
func (r *MapPhraseLinter) Expr() expr.Expr                  { return r.expression }
func (r *MapPhraseLinter) Description() string              { return r.description }
func (r *MapPhraseLinter) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *MapPhraseLinter) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: r.lintKind, Suggestions: lint.ToReplaceSuggestions(r.correctForms, span.GetContent(src)), Message: r.message, Priority: 31}
}
