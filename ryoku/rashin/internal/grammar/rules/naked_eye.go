package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type NakedEye struct{ expression expr.Expr }

func NewNakedEye() *NakedEye {
	return &NakedEye{expression: expr.NewSequenceExprOptional(expr.NewSequenceExprWordSet([]string{"hide", "hides", "hid", "hidden", "hiding", "keep", "keeps", "kept", "keeping"}).TWs().ThenOptional(expr.NewSequenceExprOptional(expr.NewSequenceExpr().ThenKindAny([]expr.KindPredicate{doc.TokenKind.IsDeterminer, doc.TokenKind.IsAdjective, doc.TokenKind.IsOov}).TWs()).ThenNoun().TWs())).ThenPreposition().TWs().TAco("the").TWs().TAco("naked").TWs().TSet([]string{"eye", "eyes"})}
}
func (r *NakedEye) Expr() expr.Expr { return r.expression }
func (r *NakedEye) Description() string {
	return "Corrects the wrong preposition used instead of `to`, `with`, or `by` the naked eye."
}
func (r *NakedEye) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *NakedEye) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	idx := 0
	hasVerb := false
	switch len(t) {
	case 7:
	case 9:
		idx = 2
		hasVerb = true
	case 11:
		idx = 4
		hasVerb = true
	case 13:
		idx = 6
		hasVerb = true
	default:
		return nil
	}
	span := t[idx].Span
	ch := span.GetContent(s)
	from := text.EqCh(ch, []rune("from"))
	if text.EqAnyIgnoreASCIICaseStr(ch, []string{"to", "with", "by"}) || hasVerb && from || !from {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Miscellaneous, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("to", ch), lint.ReplaceWithMatchCaseStr("with", ch), lint.ReplaceWithMatchCaseStr("by", ch)}, Message: "This idiom usually requires a different preposition.", Priority: 127}
}
