package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type TheProperNounPossessive struct{ expression expr.Expr }

func NewTheProperNounPossessive() *TheProperNounPossessive {
	return &TheProperNounPossessive{expression: expr.NewSequenceExprAco("the").TWs().Then(func(t doc.Token, s []rune) bool {
		k := t.Kind
		w := strings.ToLower(t.GetStr(s))
		return k.IsPossessiveNoun() && k.IsTitlecase() && !k.IsLowercase() && w != "internet's" && w != "internet’s"
	})}
}
func (r *TheProperNounPossessive) Expr() expr.Expr { return r.expression }
func (r *TheProperNounPossessive) Description() string {
	return "Checks for redundant `the` before possessive proper noun such as `The London's population`."
}
func (r *TheProperNounPossessive) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *TheProperNounPossessive) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Redundancy

	result.Span, _ = doc.Span(toks[:2])
	result.Suggestions = []lint.Suggestion{lint.Remove()}
	result.Message = "The definite article `the` is redundant before a proper noun in the possessive."
	return &result
}
