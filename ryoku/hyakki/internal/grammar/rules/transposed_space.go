package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type TransposedSpace struct {
	expression expr.Expr
	dict       dict.Dictionary
}

func NewTransposedSpace(d dict.Dictionary) *TransposedSpace {
	return &TransposedSpace{expression: expr.NewFirstMatchOf([]expr.Expr{expr.NewSequenceExpr().ThenOov().TWs().ThenOov()}), dict: d}
}
func NewTransposedSpaceSensitive(d dict.Dictionary) *TransposedSpace {
	return &TransposedSpace{expression: expr.NewFirstMatchOf([]expr.Expr{expr.NewSequenceExpr().ThenOov().TWs().ThenAnyWord(), expr.NewSequenceExprAnyWord().TWs().ThenOov(), expr.NewSequenceExpr().ThenOov().TWs().ThenOov()}), dict: d}
}
func (r *TransposedSpace) Expr() expr.Expr                  { return r.expression }
func (r *TransposedSpace) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *TransposedSpace) Description() string {
	return "Looks for a space one character too early or too late between words."
}
func (r *TransposedSpace) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	word1 := toks[0].GetCh(src)
	word2 := toks[len(toks)-1].GetCh(src)
	if len(word1) == 0 || len(word2) == 0 {
		return nil
	}
	w1Start := word1[:len(word1)-1]
	w2End := word2[1:]
	w1LastPlusW2 := make([]rune, 1, len(word2)+1)
	w1LastPlusW2[0] = word1[len(word1)-1]
	w1LastPlusW2 = append(w1LastPlusW2, word2...)
	w1PlusW2First := append(append([]rune(nil), word1...), word2[0])
	var values []string
	if r.dict.ContainsWord(w1Start) && r.dict.ContainsWord(w1LastPlusW2) {
		values = append(values, string(w1Start)+" "+string(w1LastPlusW2))
	}
	if r.dict.ContainsWord(w1PlusW2First) && r.dict.ContainsWord(w2End) {
		v := string(w1PlusW2First) + " " + string(w2End)
		if len(values) == 0 || values[0] != v {
			values = append(values, v)
		}
	}
	if len(values) == 0 {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Typo
	result.Suggestions = lint.ToReplaceSuggestions(values, span.GetContent(src))
	result.Message = fmt.Sprintf("Is the space between `%s` and `%s` one character out of place?", string(word1), string(word2))
	return &result
}
