package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type BarelyUn struct {
	expression expr.Expr
	dictionary dict.Dictionary
}

func NewBarelyUn(dictionary dict.Dictionary) *BarelyUn {
	return &BarelyUn{expression: expr.And(expr.NewSequenceExprAco("barely").TWs().ThenPositiveAdjective(), expr.NewSequenceExprAnything().TAny().Then(func(t doc.Token, s []rune) bool {
		stem, ok := text.StripPrefixIgnoreASCIICaseChars(t.GetCh(s), []rune("un"))
		if !ok {
			return false
		}
		md := dictionary.GetWordMetadata(stem)
		return md != nil && md.IsPositiveAdjective()
	})), dictionary: dictionary}
}
func (r *BarelyUn) Expr() expr.Expr { return r.expression }
func (r *BarelyUn) Description() string {
	return "Flags using `barely` with a negative adjective starting with `un-` (`barely unusable`, etc.), which is a kind of double negative."
}
func (r *BarelyUn) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *BarelyUn) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	last := t[len(t)-1]
	stem, ok := text.StripPrefixIgnoreASCIICaseChars(last.GetCh(s), []rune("un"))
	if !ok {
		return nil
	}
	result := lint.NewLint()
	result.Span = last.Span
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCase(stem, last.GetCh(s))}
	result.Message = "Using `barely` with a negative adjective is a kind of double negative"
	return &result
}
