package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type OverPlus struct{ expression expr.Expr }

func NewOverPlus() *OverPlus {
	return &OverPlus{expression: expr.ButNot(expr.NewSequenceExprAco("over").TWs().ThenAnyOf([]expr.Expr{expr.NewSpelledNumberExpr(), expr.AsExpr(func(t doc.Token, _ []rune) bool { return t.Kind.IsNumber() })}).ThenOptional(expr.NewSequenceExprWhitespace().TSet([]string{"thousand", "million", "billion"})).ThenAnyOf([]expr.Expr{expr.ButNot(expr.NewSequenceExprWhitespace().TAco("plus"), expr.NewWordExact("PLUS")), expr.NewSequenceExpr().ThenPlus()}), expr.NewSequenceExprAnything().TAny().TAco("one"))}
}
func (r *OverPlus) Expr() expr.Expr                  { return r.expression }
func (r *OverPlus) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*OverPlus) Description() string {
	return "Detected redundant use of `over` and `plus` used together to bracket a number."
}
func (r *OverPlus) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
func (r *OverPlus) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	if lint.FollowedByHyphen(ctx) || lint.FollowedByWord(ctx, func(t *doc.Token) bool {
		return text.EqAnyIgnoreASCIICaseStr(t.GetCh(src), []string{"innings", "size"})
	}) {
		return nil
	}
	if len(toks) < 3 {
		return nil
	}
	end := -2
	if toks[len(toks)-1].Kind.IsWord() {
		end--
	}
	prefix, ok := doc.GetRelSlice(toks, 0, end)
	if !ok {
		return nil
	}
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Redundancy
	l.Message = "It's redundant to use both `over` and `plus`."
	for _, part := range [][]doc.Token{prefix, toks[2:]} {
		chars, ok := doc.GetCh(part, src)
		if !ok {
			return nil
		}
		l.Suggestions = append(l.Suggestions, lint.ReplaceWithMatchCase(chars, span.GetContent(src)))
	}
	return &l
}
