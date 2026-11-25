package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type PassionateAbout struct{ expression expr.Expr }

func NewPassionateAbout() *PassionateAbout {
	return &PassionateAbout{expression: expr.NewLongestMatchOf([]expr.Expr{expr.ButNot(expr.NewSequenceExprAco("passionate").TWs().ThenPreposition(), expr.NewSequenceExprAnything().TAny().TAco("about")), expr.ButNot(expr.NewSequenceExpr().ThenPreposition().TWs().TAco("which").TWs().ThenSubjectPronoun().TWs().Then(expr.NewInflectionOfBe()).TWs().TAco("passionate").ThenOptional(expr.NewSequenceExprWhitespace().ThenPreposition()), expr.NewWord("about"))})}
}
func (r *PassionateAbout) Expr() expr.Expr                  { return r.expression }
func (r *PassionateAbout) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *PassionateAbout) Description() string {
	return "Corrects `passtionate of` to `passionate about`."
}
func (r *PassionateAbout) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
func (r *PassionateAbout) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	idx := -1
	switch len(toks) {
	case 3:
		idx = 2
	case 9:
		idx = 0
	case 11:
	default:
		return nil
	}
	result := lint.NewLint()
	result.LintKind = lint.Usage
	result.Message = "Use `about` instead of `of` with `passionate`"
	if idx >= 0 {
		result.Span = toks[idx].Span
		result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("about", toks[idx].GetCh(src))}
		return &result
	}
	if text.EqStr(toks[0].GetCh(src), "of") && text.EqStr(toks[len(toks)-1].GetCh(src), "about") && lint.PrecededByWord(ctx, func(t *doc.Token) bool { return text.EqAnyIgnoreASCIICaseStr(t.GetCh(src), []string{"all", "both"}) }) {
		return nil
	}
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	first, ok := doc.Span(toks[1:9])
	if !ok {
		return nil
	}
	last, ok := doc.Span(toks[2:10])
	if !ok {
		return nil
	}
	result.Span = span
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("about"+first.GetContentString(src), span.GetContent(src)), lint.ReplaceWithMatchCaseStr(last.GetContentString(src)+"about", span.GetContent(src))}
	return &result
}
