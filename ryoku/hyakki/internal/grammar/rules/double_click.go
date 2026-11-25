package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type DoubleClick struct{ expression expr.Expr }

func NewDoubleClick() *DoubleClick {
	return &DoubleClick{expression: expr.NewFirstMatchOf([]expr.Expr{doubleClickSequence().TWs().ThenAnyWord(), doubleClickSequence().ThenPunctuation(), doubleClickSequence().TWs().ThenKindIsButIsNot(doc.TokenKind.IsWord, doc.TokenKind.IsVerb)})}
}
func (r *DoubleClick) Expr() expr.Expr                  { return r.expression }
func (r *DoubleClick) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *DoubleClick) Description() string {
	return "Encourages hyphenating `double-click` and its inflections."
}
func (r *DoubleClick) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 3 {
		return nil
	}
	span, _ := doc.Span(toks[:3])
	replacement := append([]rune(nil), toks[0].GetCh(src)...)
	replacement = append(replacement, '-')
	replacement = append(replacement, toks[2].GetCh(src)...)
	return &lint.Lint{Span: span, LintKind: lint.Punctuation, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCase(replacement, span.GetContent(src))}, Message: "Add a hyphen to this command.", Priority: 40}
}
func doubleClickSequence() *expr.SequenceExpr {
	return expr.NewSequenceExprAco("double").TWs().TSet([]string{"click", "clicked", "clicking", "clicks"})
}
