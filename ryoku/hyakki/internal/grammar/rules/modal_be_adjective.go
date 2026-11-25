package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ModalBeAdjective struct{ expression expr.Expr }

func NewModalBeAdjective() *ModalBeAdjective {
	return &ModalBeAdjective{expr.NewSequenceExprWith(expr.NewModalVerb()).TWs().ThenKindIsButIsntAnyOfExcept(doc.TokenKind.IsAdjective, []expr.KindPredicate{doc.TokenKind.IsVerbLemma, doc.TokenKind.IsAdverb, doc.TokenKind.IsPreposition, doc.TokenKind.IsDeterminer, doc.TokenKind.IsPronoun}, []string{"backup", "likely", "way"})}
}
func (r *ModalBeAdjective) Expr() expr.Expr                  { return r.expression }
func (r *ModalBeAdjective) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*ModalBeAdjective) Description() string {
	return "Looks for `be` missing between a modal verb and adjective."
}
func (r *ModalBeAdjective) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return r.MatchToLintWithContext(t, s, nil)
}
func (r *ModalBeAdjective) MatchToLintWithContext(t []doc.Token, s []rune, c *lint.MatchContext) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	if lint.FollowedByWord(c, func(n *doc.Token) bool {
		v := n.GetCh(s)
		return (n.Kind.IsNoun() && !text.EqAnyIgnoreASCIICaseStr(v, []string{"at", "by", "if"})) || (text.EqStr(t[len(t)-1].GetCh(s), "kind") && text.EqStr(v, "of"))
	}) || lint.FollowedByHyphen(c) {
		return nil
	}
	return &lint.Lint{Span: t[0].Span, LintKind: lint.Miscellaneous, Suggestions: []lint.Suggestion{lint.InsertAfter([]rune(" be"))}, Message: "You may be missing the word `be` between this modal verb and adjective.", Priority: 127}
}
