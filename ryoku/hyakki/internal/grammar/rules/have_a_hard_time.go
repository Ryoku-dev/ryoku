package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type HaveAHardTime struct {
	expression expr.Expr
	dialect    dict.Dialect
}

func NewHaveAHardTime(dialect dict.Dialect) *HaveAHardTime {
	return &HaveAHardTime{expression: expr.NewSequenceExprWordSet([]string{"have", "had", "has", "having"}).ThenOptional(expr.NewSequenceExprWhitespace().TSet([]string{"extremely", "real"})).TWs().ThenWordSeq([]string{"hard", "time"}), dialect: dialect}
}
func (r *HaveAHardTime) Expr() expr.Expr { return r.expression }
func (r *HaveAHardTime) Description() string {
	return "Corrects `have hard time` to `have a hard time`."
}
func (r *HaveAHardTime) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *HaveAHardTime) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) < 3 {
		return nil
	}
	sound, ok := dict.StartsWithVowel(t[2].GetCh(s), r.dialect)
	if !ok {
		return nil
	}
	var suggestions []lint.Suggestion
	switch sound {
	case dict.Consonant:
		suggestions = []lint.Suggestion{lint.InsertAfter([]rune("a "))}
	case dict.Vowel:
		suggestions = []lint.Suggestion{lint.InsertAfter([]rune("an "))}
	case dict.Either:
		suggestions = []lint.Suggestion{lint.InsertAfter([]rune("a ")), lint.InsertAfter([]rune("an "))}
	default:
		return nil
	}
	return &lint.Lint{Span: t[1].Span, LintKind: lint.Usage, Suggestions: suggestions, Message: "This idiom requires the indefinite article `a`.", Priority: 127}
}
