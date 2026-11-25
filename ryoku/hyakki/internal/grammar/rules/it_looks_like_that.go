package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type ItLooksLikeThat struct{ expression expr.Expr }

func NewItLooksLikeThat() *ItLooksLikeThat {
	return &ItLooksLikeThat{expression: expr.NewSequenceExprWordSeq([]string{"it", "looks", "like", "that"}).TWs().ThenKindWhere(func(k doc.TokenKind) bool {
		return k.IsSubjectPronoun() || k.IsVerbProgressiveForm() || !(k.IsVerbThirdPersonSingularPresentForm() || k.IsAuxiliaryVerb() || k.IsVerbSimplePastForm() || k.IsNoun() || k.IsOov())
	})}
}
func (r *ItLooksLikeThat) Expr() expr.Expr                  { return r.expression }
func (r *ItLooksLikeThat) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ItLooksLikeThat) Description() string {
	return "Corrects `it looks like that` to just `it looks like`."
}
func (r *ItLooksLikeThat) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 8 {
		return nil
	}
	span, _ := doc.Span(toks[6:8])
	return &lint.Lint{Span: span, LintKind: lint.Redundancy, Suggestions: []lint.Suggestion{lint.Remove()}, Message: "`that` is redundant and ungrammatical here", Priority: 31}
}
