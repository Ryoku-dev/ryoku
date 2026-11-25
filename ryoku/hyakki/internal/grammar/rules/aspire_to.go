package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type AspireTo struct{ expression expr.Expr }

func NewAspireTo() *AspireTo {
	return &AspireTo{expression: expr.NewSequenceExprWordSet([]string{"aspire", "aspired", "aspires", "aspiring"}).TWs().TAco("for")}
}
func (r *AspireTo) Expr() expr.Expr                  { return r.expression }
func (r *AspireTo) Description() string              { return "Corrects `aspire for` to `aspire to`." }
func (r *AspireTo) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *AspireTo) Unit() lint.DocumentIterator      { return lint.Sentence{} }
func (r *AspireTo) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
func (r *AspireTo) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	if toks[0].GetStr(src) == "Aspire" && !lint.AtStartOfSentence(ctx) {
		return nil
	}
	if lint.PrecededByWord(ctx, func(t *doc.Token) bool {
		if t.Kind.IsPreposition() || t.Kind.IsUnlintable() {
			return true
		}
		return t.GetStr(src) == "NET" || text.EqAnyIgnoreASCIICaseStr(t.GetCh(src), []string{"use", "used", "uses", "using", "dotnet"})
	}) || lint.FollowedByWord(ctx, func(t *doc.Token) bool { return t.GetStr(src) == "AWS" }) {
		return nil
	}
	span := toks[2].Span
	suggestions := []lint.Suggestion{lint.ReplaceWithMatchCaseStr("to", src[span.Start:span.End])}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Usage
	result.Message = "Use `aspire to` instead of `aspire for`."
	result.Suggestions = suggestions
	return &result
}
