package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type WrongNegative struct {
	expression expr.Expr
	dictionary dict.Dictionary
}

func NewWrongNegative(dictionary dict.Dictionary) *WrongNegative {
	return &WrongNegative{expression: expr.AsExpr(func(t doc.Token, s []rune) bool {
		return t.Kind.IsOov() && text.StartsWithAnyIgnoreASCIICaseStr(t.GetCh(s), wrongNegativePrefixes) && !text.EqAnyIgnoreASCIICaseStr(t.GetCh(s), []string{"defo", "deus"})
	}), dictionary: dictionary}
}
func (r *WrongNegative) Expr() expr.Expr { return r.expression }
func (r *WrongNegative) Description() string {
	return "If an unknown word looks like it might be a negative word, suggests correct words that are in the dictionary."
}
func (r *WrongNegative) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *WrongNegative) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) != 1 {
		return nil
	}
	span := t[0].Span
	word := t[0].GetCh(s)
	suggestions := []lint.Suggestion{}
	for _, prefix := range wrongNegativePrefixes {
		if !text.StartsWithIgnoreASCIICaseStr(word, prefix) || len(word) <= len(prefix) {
			continue
		}
		rest := word[len(prefix):]
		for _, p := range wrongNegativePrefixes {
			if p == prefix {
				continue
			}
			candidate := append([]rune(p), rest...)
			if r.dictionary.ContainsWord(candidate) {
				suggestions = append(suggestions, lint.ReplaceWithMatchCase(candidate, word))
			}
		}
	}
	if len(suggestions) == 0 {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.WordChoice
	result.Suggestions = suggestions
	result.Priority = 62
	result.Message = "Could one of these be the negative word you intended?"
	if len(suggestions) == 1 {
		result.Message = "Could this be the negative word you intended?"
	}
	return &result
}

var wrongNegativePrefixes = []string{"anti", "de", "dis", "dys", "il", "im", "in", "non", "un"}
