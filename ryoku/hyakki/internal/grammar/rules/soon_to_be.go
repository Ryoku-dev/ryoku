package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type SoonToBe struct{ expression *expr.ExprMap[text.Span] }

func NewSoonToBe() *SoonToBe {
	soon := func() *expr.SequenceExpr { return expr.NewSequenceExprAco("soon").TWs().TAco("to").TWs().TAco("be") }
	trailing := func() *expr.SequenceExpr {
		nominal := expr.NewSequenceExprOptional(expr.NewSequenceExpr().ThenOneOrMoreAdverbs().TWs()).Then(expr.NewNominalPhrase())
		number := expr.NewSequenceExprNumber().ThenHyphen().ThenNominal().ThenOptional(expr.NewSequenceExpr().ThenHyphen().ThenAdjective()).TWs().ThenNominal()
		compound := expr.NewSequenceExpr().ThenKindAny([]expr.KindPredicate{doc.TokenKind.IsWordLike}).ThenHyphen().ThenNominal()
		return expr.NewSequenceExprAnyOf([]expr.Expr{number, compound, nominal})
	}
	m := expr.NewExprMap[text.Span]()
	m.Insert(expr.NewSequenceExpr().ThenDeterminer().TWs().ThenSeq(soon()).TWs().ThenSeq(trailing()), text.Span{Start: 2, End: 7})
	m.Insert(expr.NewSequenceExpr().ThenSeq(soon()).TWs().ThenSeq(trailing()), text.Span{Start: 0, End: 5})
	return &SoonToBe{expression: m}
}
func (r *SoonToBe) Expr() expr.Expr                  { return r.expression }
func (r *SoonToBe) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *SoonToBe) Description() string {
	return "Hyphenates `soon-to-be` when it appears before a noun."
}
func (r *SoonToBe) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	rg, ok := r.expression.Lookup(0, toks, src)
	if !ok || rg.End > len(toks) {
		return nil
	}
	span, ok := doc.Span(toks[rg.Start:rg.End])
	if !ok {
		return nil
	}
	found := false
	for _, tok := range toks[rg.End:] {
		if tok.Kind.IsWhitespace() || tok.Kind.IsHyphen() {
			continue
		}
		if tok.Kind.IsPunctuation() {
			break
		}
		if tok.Kind.IsNominal() {
			if tok.Kind.IsPreposition() {
				continue
			}
			found = true
			break
		}
	}
	if !found {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Miscellaneous, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("soon-to-be", span.GetContent(src))}, Message: "Use hyphens when `soon to be` modifies a noun.", Priority: 31}
}
