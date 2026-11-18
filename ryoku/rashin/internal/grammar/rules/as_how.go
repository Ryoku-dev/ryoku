package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type AsHow struct{ expression expr.Expr }

func NewAsHow() *AsHow {
	return &AsHow{expression: expr.NewSequenceExprWordSeq([]string{"as", "how"})}
}
func (r *AsHow) Expr() expr.Expr                  { return r.expression }
func (r *AsHow) Description() string              { return "Corrects `as how` to `as to how`." }
func (r *AsHow) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *AsHow) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
func (r *AsHow) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	if ctx != nil && len(ctx.Before) >= 2 {
		before := ctx.Before
		last := before[len(before)-2]
		if before[len(before)-1].Kind.IsWhitespace() {
			if text.EqStr(last.GetCh(src), "same") {
				return nil
			}
			if len(before) >= 4 && last.Kind.IsAdjective() && before[len(before)-3].Kind.IsWhitespace() && text.EqCh(before[len(before)-4].GetCh(src), []rune("as")) {
				return nil
			}
		}
	}
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	suggestions := []lint.Suggestion{lint.ReplaceWithMatchCaseStr("as to how", src[span.Start:span.End])}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Usage
	result.Message = "Consider rephrasing to 'as to how'"
	result.Suggestions = suggestions
	return &result
}
