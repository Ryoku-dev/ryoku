package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type BestOfAllTime struct{ expression expr.Expr }

func NewBestOfAllTime() *BestOfAllTime {
	return &BestOfAllTime{expression: expr.NewSequenceExprAnyOf([]expr.Expr{expr.NewSequenceExpr().ThenSuperlativeAdjective(), expr.NewSequenceExprAco("most").TWs().ThenPositiveAdjective(), expr.NewSequenceExprWordSet([]string{"favorite", "favourite", "top"})}).ThenZeroOrMore(func(t doc.Token, _ []rune) bool { return !t.Kind.IsNoun() && !t.Kind.IsOov() }).ThenKindWhere(func(k doc.TokenKind) bool { return k.IsNoun() || k.IsOov() }).ThenZeroOrMore(expr.NewSequenceExprWhitespace().ThenKindWhere(func(k doc.TokenKind) bool { return k.IsNoun() || k.IsOov() })).ThenFixedPhrase(" of all times")}
}
func (r *BestOfAllTime) Expr() expr.Expr { return r.expression }
func (r *BestOfAllTime) Description() string {
	return "Checks for nonstandard `of all times` in superlatives instead of singular `time`"
}
func (r *BestOfAllTime) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *BestOfAllTime) Unit() lint.DocumentIterator      { return lint.Sentence{} }
func (r *BestOfAllTime) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	span := toks[len(toks)-1].Span
	content := src[span.Start:span.End]
	if len(content) == 0 {
		return nil
	}
	suggestions := []lint.Suggestion{lint.ReplaceWith(append([]rune(nil), content[:len(content)-1]...))}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.WordChoice
	result.Message = "This expression uses singular `time`"
	result.Suggestions = suggestions
	return &result
}
