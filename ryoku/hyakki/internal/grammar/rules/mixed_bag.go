package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type MixedBag struct{ expression expr.Expr }

func NewMixedBag() *MixedBag {
	return &MixedBag{expression: expr.NewSequenceExpr().ThenKindAnyOrWords([]expr.KindPredicate{doc.TokenKind.IsAdjective, doc.TokenKind.IsAdverb}, []string{"a"}).TWs().TAco("mixed").TWs().TAco("bad")}
}
func (r *MixedBag) Expr() expr.Expr                  { return r.expression }
func (r *MixedBag) Description() string              { return "Corrects the eggcorn `mixed bad` to `mixed bag`." }
func (r *MixedBag) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *MixedBag) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	bad := lint.FindTheOnlyTokenMatching(t, s, func(t *doc.Token, s []rune) bool { return text.EqCh(t.GetCh(s), []rune("bad")) })
	if bad == nil {
		return nil
	}
	return &lint.Lint{Span: bad.Span, LintKind: lint.Eggcorn, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("bag", bad.GetCh(s))}, Message: "Corrects the eggcorn `mixed bad` to `mixed bag`.", Priority: 127}
}
