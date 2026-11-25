package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type NeedToNoun struct{ expression expr.Expr }

func NewNeedToNoun() *NeedToNoun {
	postfix := expr.NewLongestMatchOf([]expr.Expr{expr.AsExpr(func(t doc.Token, _ []rune) bool {
		return t.Kind.IsAdverb() || t.Kind.IsDeterminer() || t.Kind.IsUnlintable() || t.Kind.IsPronoun()
	}), expr.AsExpr(expr.NewWordSet([]string{"about", "into", "it"}))})
	exceptions := expr.NewSequenceExprAnything().TAny().TAny().TAny().ThenWordSet([]string{"be", "match"})
	a := expr.NewSequenceExpr().ThenKindWhere(func(k doc.TokenKind) bool { return k.IsNominal() && !k.IsLikelyHomograph() }).TWs().ThenUnless(postfix)
	b := expr.NewSequenceExpr().ThenKindWhere(func(k doc.TokenKind) bool { return k.IsNominal() && !k.IsVerb() && !k.IsLikelyHomograph() })
	e := expr.NewSequenceExprWith(expr.NewDerivedFromStr("need")).TWs().TAco("to").TWs().Then(expr.Or(a, b))
	return &NeedToNoun{expression: expr.And(e, expr.NewUnlessStep(exceptions, func(doc.Token, []rune) bool { return true }))}
}
func (r *NeedToNoun) Expr() expr.Expr { return r.expression }
func (r *NeedToNoun) Description() string {
	return "Flags `need to` when it is immediately followed by a noun, which usually means the infinitive verb is missing."
}
func (r *NeedToNoun) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *NeedToNoun) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) < 5 {
		return nil
	}
	return &lint.Lint{Span: t[2].Span, LintKind: lint.Grammar, Suggestions: []lint.Suggestion{lint.ReplaceWith([]rune("the"))}, Message: fmt.Sprintf("`need to` should be followed by a verb, not a noun or pronoun like `%s`.", t[4].GetStr(s)), Priority: 48}
}
