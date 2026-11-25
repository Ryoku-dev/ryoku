package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"strings"
)

type QuantifierNumeralConflict struct{ expression expr.Expr }

func NewQuantifierNumeralConflict() *QuantifierNumeralConflict {
	return &QuantifierNumeralConflict{expression: expr.NewAll([]expr.Expr{expr.NewSequenceExpr().ThenQuantifier().TWs().ThenLongestOf([]expr.Expr{expr.NewSpelledNumberExpr(), expr.NewSequenceExpr().ThenCardinalNumber()}), expr.NewSequenceExprUnless(expr.NewSequenceExprAnyOf([]expr.Expr{expr.NewWordSet([]string{"all", "any", "every", "no"}), expr.NewSequenceExprWordSet([]string{"each", "some"}).TWs().TAco("one")}))})}
}
func (r *QuantifierNumeralConflict) Expr() expr.Expr { return r.expression }
func (r *QuantifierNumeralConflict) Lint(d *doc.Document) []lint.Lint {
	return lint.RunExprLinter(r, d)
}
func (r *QuantifierNumeralConflict) Description() string {
	return "Detects quantifier-numeral conflicts"
}
func (r *QuantifierNumeralConflict) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	if ctx != nil && len(ctx.After) > 0 && ctx.After[0].Kind.IsHyphen() {
		return nil
	}
	quant := toks[0].GetStr(src)
	switch strings.ToLower(quant) {
	case "least", "most":
		if ctx != nil && len(ctx.Before) >= 2 {
			prev := ctx.Before[len(ctx.Before)-2:]
			if prev[1].Kind.IsWhitespace() && prev[0].Kind.IsWord() && text.EqStr(prev[0].GetCh(src), "at") {
				return nil
			}
		}
	case "each":
		return &lint.Lint{Span: toks[0].Span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("every", toks[0].GetCh(src))}, Message: "Use 'every' instead of 'each' before a number.", Priority: 127}
	case "both":
		if ctx != nil {
			if span, ok := expr.NewNominalPhrase().Run(1, ctx.After, src); ok && span.End+1 < len(ctx.After) {
				following := ctx.After[span.End:]
				if following[0].Kind.IsWhitespace() && following[1].Kind.IsConjunction() && strings.EqualFold(following[1].GetStr(src), "and") {
					return nil
				}
			}
		}
	}
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Grammar, Message: "The word '" + quant + "' should not be used before a number.", Priority: 127}
}
func (r *QuantifierNumeralConflict) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
