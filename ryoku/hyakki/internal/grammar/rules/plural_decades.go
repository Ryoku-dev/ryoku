package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type PluralDecades struct{ expression expr.Expr }

func NewPluralDecades() *PluralDecades {
	return &PluralDecades{expression: expr.NewSequenceExpr().ThenCardinalNumber().ThenApostrophe().TAco("s")}
}
func (r *PluralDecades) Expr() expr.Expr                  { return r.expression }
func (r *PluralDecades) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*PluralDecades) Description() string {
	return "Flags plural decades erroneously using an apostrophe before the `s`"
}
func (r *PluralDecades) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return r.MatchToLintWithContext(t, s, nil)
}
func (*PluralDecades) Unit() lint.DocumentIterator { return lint.Sentence{} }
func (*PluralDecades) MatchToLintWithContext(t []doc.Token, s []rune, ctx *lint.MatchContext) *lint.Lint {
	if len(t) != 3 {
		return nil
	}
	decade, suffix := t[0].GetCh(s), t[2].GetCh(s)
	if (len(decade) != 2 && len(decade) != 4) || decade[len(decade)-1] != '0' {
		return nil
	}
	var before, after []doc.Token
	if ctx != nil {
		before, after = ctx.Before, ctx.After
	}
	if len(decade) == 4 {
		return pluralDecadesFourDigitsMatchToLint(t, s, decade, suffix, before, after)
	}
	return pluralDecadesTwoDigitsMatchToLint(t, s, decade, suffix, before, after)
}
