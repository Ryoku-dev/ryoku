package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/pos"
)

type ItsPossessive struct{ expression *expr.ExprMap[int] }

func NewItsPossessive() *ItsPossessive {
	m := expr.NewExprMap[int]()
	noun := func() expr.Expr {
		return expr.Or(expr.NewUPOSSet([]pos.UPOS{pos.NOUN, pos.PROPN}), func(t doc.Token, _ []rune) bool { n, ok := t.Kind.AsNumber(); return ok && n.Suffix != nil })
	}
	exclusion := func() *expr.UPOSSet {
		return expr.NewUPOSSet([]pos.UPOS{pos.VERB, pos.PART, pos.ADP, pos.NOUN, pos.PRON, pos.SCONJ, pos.CCONJ, pos.ADV})
	}
	m.Insert(expr.NewSequenceExprWith(expr.NewUPOSSet([]pos.UPOS{pos.VERB, pos.ADP})).TWs().TAco("it's").ThenOptional(expr.NewSequenceExpr().TWs().Then(expr.NewUPOSSet([]pos.UPOS{pos.ADJ}))).TWs().Then(noun()), 2)
	m.Insert(expr.NewSequenceExprWith(expr.NewAnchorStart()).TAco("it's").TWs().Then(noun()).ThenUnless(expr.NewSequenceExpr().TWs().Then(exclusion())), 0)
	subjectNoun := expr.Or(expr.NewUPOSSet([]pos.UPOS{pos.NOUN, pos.PROPN}), func(t doc.Token, _ []rune) bool {
		n, ok := t.Kind.AsNumber()
		return (ok && n.Suffix != nil) || ((t.Kind.IsNoun() || t.Kind.IsProperNoun()) && !t.Kind.IsAdjective() && !t.Kind.IsAdverb())
	})
	m.Insert(expr.NewSequenceExprWith(expr.NewAnchorStart()).TAco("it's").TWs().Then(subjectNoun).TWs().Then(expr.NewUPOSSet([]pos.UPOS{pos.VERB, pos.AUX})), 0)
	m.Insert(expr.NewSequenceExprWith(expr.NewAnchorStart()).TAco("it's").TWs().Then(expr.NewUPOSSet([]pos.UPOS{pos.ADJ})).TWs().ThenUnless(exclusion()), 0)
	m.Insert(expr.NewSequenceExprWith(expr.NewAnchorStart()).Then(expr.NewUPOSSet([]pos.UPOS{pos.CCONJ})).TWs().TAco("it's").TWs().Then(noun()).ThenUnless(expr.NewSequenceExpr().TWs().Then(exclusion())), 2)
	m.Insert(expr.NewSequenceExprWordSeq([]string{"it's", "various"}), 0)
	return &ItsPossessive{m}
}
func (r *ItsPossessive) Expr() expr.Expr                  { return r.expression }
func (r *ItsPossessive) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*ItsPossessive) Description() string {
	return "In English, possessive pronouns never take an apostrophe. Use `its` to show ownership (e.g. “its texture”) and avoid confusing it with `it's`, which always means “it is” or “it has.”"
}
func (r *ItsPossessive) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	idx, ok := r.expression.Lookup(0, t, s)
	if !ok || idx >= len(t) {
		return nil
	}
	w := t[idx]
	return &lint.Lint{Span: w.Span, LintKind: lint.Agreement, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("its", w.GetCh(s))}, Message: "Use the possessive pronoun `its` (without an apostrophe) to show ownership. The word `it's` (with an apostrophe) is a contraction of 'it is' or 'it has' and should not be used to indicate possession.", Priority: 31}
}
