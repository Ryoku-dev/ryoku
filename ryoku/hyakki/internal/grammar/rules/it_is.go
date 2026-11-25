package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type ItIs struct{ expression expr.Expr }

func NewItIs() *ItIs {
	return &ItIs{expression: expr.NewSequenceExprAco("its").ThenWhitespace().ThenKindExcept(doc.TokenKind.IsAdjective, []string{"1st", "animal", "assault", "body", "budget", "business", "center", "centre", "frontline", "head", "key", "mainline", "material", "mean", "own", "power", "regulation", "runtime", "size", "state", "team", "turnover", "utility", "woman"}).ThenWhitespace().ThenPreposition()}
}
func (r *ItIs) Expr() expr.Expr                  { return r.expression }
func (r *ItIs) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ItIs) Description() string {
	return "Detects when “its” is used before an adjective + preposition and suggests the contraction “it's”."
}
func (r *ItIs) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	return &lint.Lint{Span: toks[0].Span, LintKind: lint.Miscellaneous, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("it's", toks[0].GetCh(src))}, Message: "Consider using 'it's' (it is) instead of 'its' (possessive form).", Priority: 31}
}
