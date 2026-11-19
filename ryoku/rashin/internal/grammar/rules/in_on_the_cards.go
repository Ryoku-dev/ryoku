package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type InOnTheCards struct {
	expression expr.Expr
	dialect    dict.Dialect
}

func NewInOnTheCards(dialect dict.Dialect) *InOnTheCards {
	prep := "on"
	if dialect == dict.British {
		prep = "in"
	}
	pre := expr.NewFirstMatchOf([]expr.Expr{expr.AsExpr(expr.NewInflectionOfBe()), expr.AsExpr(expr.NewWordSet([]string{"isn't", "it's", "wasn't", "weren't", "not", "isnt", "its", "wasnt", "werent"}))})
	return &InOnTheCards{expression: expr.NewSequenceExprWith(pre).TWs().TAco(prep).ThenFixedPhrase(" the cards"), dialect: dialect}
}
func (r *InOnTheCards) Expr() expr.Expr { return r.expression }
func (r *InOnTheCards) Description() string {
	return "Corrects either `in the cards` or `on the cards` to the other, depending on the dialect."
}
func (r *InOnTheCards) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *InOnTheCards) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) < 3 {
		return nil
	}
	prep := t[2].GetCh(s)
	if len(prep) < 2 {
		return nil
	}
	replacement := []rune{0, prep[1]}
	switch prep[0] {
	case 'i':
		replacement[0] = 'o'
	case 'o':
		replacement[0] = 'i'
	case 'I':
		replacement[0] = 'O'
	case 'O':
		replacement[0] = 'I'
	default:
		return nil
	}
	return &lint.Lint{Span: t[2].Span, LintKind: lint.Regionalism, Suggestions: []lint.Suggestion{lint.ReplaceWith(replacement)}, Message: fmt.Sprintf("Use `%s the cards` instead of `%s the cards` in %s English.", string(replacement), string(prep), r.dialect), Priority: 63}
}
