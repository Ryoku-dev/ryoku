package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type OutOfOutsideTheBox struct{ expression expr.Expr }

func NewOutOfOutsideTheBox() *OutOfOutsideTheBox {
	return &OutOfOutsideTheBox{expression: expr.NewSequenceExprAnyOf([]expr.Expr{expr.NewSequenceExprAnyOf([]expr.Expr{expr.NewSequenceExprAco("out").TWs().TAco("side"), expr.NewSequenceExprAco("outside")}).ThenOptional(expr.NewSequenceExprWhitespace().TAco("of")), expr.NewSequenceExprAco("out").TWs().TAco("of")}).TWs().ThenWordSeq([]string{"the", "box"})}
}
func (r *OutOfOutsideTheBox) Expr() expr.Expr { return r.expression }
func (r *OutOfOutsideTheBox) Description() string {
	return "Checks for incorrect mixing of the idioms `think outside the box` and `out of the box`."
}
func (r *OutOfOutsideTheBox) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *OutOfOutsideTheBox) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
func (r *OutOfOutsideTheBox) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	var selected []doc.Token
	replacement := ""
	message := ""
	if text.EqCh(toks[2].GetCh(src), []rune("of")) && lint.PrecededByWord(ctx, func(t *doc.Token) bool {
		return text.EqAnyIgnoreASCIICaseStr(t.GetCh(src), []string{"think", "thinking", "thinks", "thought"})
	}) {
		selected = toks[:3]
		replacement = "outside"
		message = "Use `outside the box` when referring to creative or original thinking."
	} else if lint.PrecededByWord(ctx, func(t *doc.Token) bool {
		return text.EqAnyIgnoreASCIICaseStr(t.GetCh(src), []string{"right", "straight", "work", "worked", "working", "works"})
	}) {
		var ok bool
		selected, ok = doc.GetRelSlice(toks, 0, -5)
		if !ok {
			return nil
		}
		replacement = "out of"
		message = "Use `out of the box` when referring to something that works immediately without setup or customization."
	} else {
		return nil
	}
	span, ok := doc.Span(selected)
	if !ok {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Usage
	result.Message = message
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(replacement, src[span.Start:span.End])}
	return &result
}
