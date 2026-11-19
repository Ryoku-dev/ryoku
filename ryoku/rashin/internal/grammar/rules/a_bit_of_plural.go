package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ABitOfPlural struct{ expression expr.Expr }

func NewABitOfPlural() *ABitOfPlural {
	return &ABitOfPlural{expression: expr.NewSequenceExprWordSet([]string{"a", "an"}).TWs().ThenOptional(expr.NewSequenceExprOptional(expr.NewSequenceExprAco("very").TWs()).TSet(aBitOfPluralLittleEtc).TWs()).ThenWordSeq([]string{"bit", "of"}).TWs().ThenOptional(expr.NewSequenceExpr().ThenAdjective().TWs()).ThenPluralNoun()}
}
func (r *ABitOfPlural) Expr() expr.Expr                  { return r.expression }
func (r *ABitOfPlural) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*ABitOfPlural) Description() string                { return "Flags `a bit of` used with plural nouns." }
func (r *ABitOfPlural) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
func (r *ABitOfPlural) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	if lint.FollowedByHyphen(ctx) {
		return nil
	}
	bit, ok := lint.FindTheOnlyTokenIndexMatching(toks, src, func(t *doc.Token, s []rune) bool { return text.EqStr(t.GetCh(s), "bit") })
	if !ok {
		return nil
	}
	start := bit
	if bit >= 4 && text.EqAnyIgnoreASCIICaseStr(toks[bit-2].GetCh(src), aBitOfPluralLittleEtc) && toks[bit-1].Kind.IsWhitespace() && toks[bit-3].Kind.IsWhitespace() && text.EqStr(toks[bit-4].GetCh(src), "very") {
		start -= 4
	} else if bit >= 2 && text.EqAnyIgnoreASCIICaseStr(toks[bit-2].GetCh(src), aBitOfPluralLittleEtc) && toks[bit-1].Kind.IsWhitespace() {
		start -= 2
	}
	if bit+2 >= len(toks) {
		return nil
	}
	span, ok := doc.Span(toks[start : bit+3])
	if !ok {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Grammar
	l.Message = "`A bit of` can only be used with mass nouns, with plural nouns use `a couple of`, etc."
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("couple of", span.GetContent(src)), lint.ReplaceWithMatchCaseStr("few", span.GetContent(src))}
	return &l
}

var aBitOfPluralLittleEtc = []string{"little", "tiny", "small"}
