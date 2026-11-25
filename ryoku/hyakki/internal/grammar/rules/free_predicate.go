package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type FreePredicate struct{ expression expr.Expr }

func NewFreePredicate() *FreePredicate                    { return &FreePredicate{expression: freePredicateExpr()} }
func (r *FreePredicate) Expr() expr.Expr                  { return r.expression }
func (r *FreePredicate) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*FreePredicate) Description() string {
	return "Helps swap in `free` when a linking verb is followed by the noun `fee`."
}
func (r *FreePredicate) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	m := r.expression.(*expr.ExprMap[int])
	idx, ok := m.Lookup(0, toks, src)
	if !ok || idx >= len(toks) {
		return nil
	}
	if len(toks) == 0 {
		return nil
	}
	span := toks[idx].Span
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.WordChoice
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("free", span.GetContent(src))}
	result.Message = "Use `free` here to show that something costs nothing."
	result.Priority = 38
	return &result
}

func freePredicateExpr() *expr.ExprMap[int] {
	m := expr.NewExprMap[int]()
	m.Insert(expr.NewSequenceExprWith(freePredicateLinkingLike).TWs().Then(freePredicateMatchesFee).ThenOptionalWhitespace().Then(freePredicateFollowsFee), 2)
	m.Insert(expr.NewSequenceExprWith(freePredicateLinkingLike).TWs().ThenAdverb().TWs().Then(freePredicateMatchesFee).ThenOptionalWhitespace().Then(freePredicateFollowsFee), 4)
	return m
}
func freePredicateMatchesFee(t doc.Token, src []rune) bool {
	return t.Kind.IsNoun() && text.EqStr(t.GetCh(src), "fee")
}
func freePredicateFollowsFee(t doc.Token, _ []rune) bool {
	return !t.Kind.IsHyphen() && (t.Kind.IsPreposition() || t.Kind.IsConjunction() || t.Kind.Kind == doc.Punctuation)
}
func freePredicateLinkingLike(t doc.Token, src []rune) bool {
	return text.EqAnyIgnoreASCIICaseStr(t.GetCh(src), []string{"be", "is", "am", "are", "was", "were", "being", "been"})
}
