package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type ByExtension struct{ expression expr.Expr }

func NewByExtension() *ByExtension {
	return &ByExtension{expression: expr.NewSequenceExprAnyOf([]expr.Expr{expr.NewSequenceExprWordSet([]string{"and", "so", "which", "thus"}).TWs(), expr.NewAnchorStart()}).ThenWordSeq([]string{"by", "extent"})}
}
func (r *ByExtension) Expr() expr.Expr                  { return r.expression }
func (r *ByExtension) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*ByExtension) Description() string                { return "Corrects `by extent` to `by extension`." }
func (r *ByExtension) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	span := toks[len(toks)-1].Span
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Miscellaneous
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("extension", span.GetContent(src))}
	result.Message = "The idiom is `by extension` not `by extent`."
	result.Priority = 127
	return &result
}
