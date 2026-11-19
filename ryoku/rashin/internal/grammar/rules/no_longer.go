package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type NoLonger struct{ expression expr.Expr }

func NewNoLonger() *NoLonger {
	return &NoLonger{expression: expr.ButNot(expr.NewSequenceExprWordSeq([]string{"not", "longer"}).ThenOptional(expr.NewSequenceExpr().TWs().ThenKindAny([]expr.KindPredicate{doc.TokenKind.IsVerbLemma, doc.TokenKind.IsVerbThirdPersonSingularPresentForm, doc.TokenKind.IsVerbPastParticipleForm, doc.TokenKind.IsVerbProgressiveForm, doc.TokenKind.IsAdjective})), expr.NewSequenceExprAnything().TAny().TAny().TAny().TAco("than"))}
}
func (r *NoLonger) Expr() expr.Expr                  { return r.expression }
func (r *NoLonger) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *NoLonger) Description() string {
	return "Corrects `not longer` when it should be `no longer`."
}
func (r *NoLonger) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	return &lint.Lint{Span: toks[0].Span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("no", toks[0].GetCh(src))}, Message: "The correct expression is `no longer`.", Priority: 127}
}
