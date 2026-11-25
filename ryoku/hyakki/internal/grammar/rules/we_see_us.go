package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type WeSeeUs struct{ expression expr.Expr }

func NewWeSeeUs() *WeSeeUs {
	return &WeSeeUs{expression: expr.NewSequenceExprAco("we").TWs().ThenAnyOf([]expr.Expr{expr.AsExpr(expr.NewWordSet([]string{"see", "saw"})), expr.NewSequenceExprWordSet([]string{"can", "could", "may", "might", "must", "shall", "should", "will", "would", "can't", "couldn't", "mightn't", "mustn't", "shan't", "shouldn't", "won't", "wouldn't", "cannot"}).TWs().TAco("see"), expr.NewSequenceExprWordSet([]string{"have", "had"}).TWs().TAco("seen"), expr.NewSequenceExprWordSeq([]string{"are", "seeing"})}).TWs().TAco("us")}
}
func (r *WeSeeUs) Expr() expr.Expr { return r.expression }
func (r *WeSeeUs) Description() string {
	return "Looks for the common Germanic phrase `we see us` and suggests `we see each other` as a replacement."
}
func (r *WeSeeUs) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *WeSeeUs) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Miscellaneous

	tok := toks[len(toks)-1]
	if tok.GetStr(src) == "US" {
		return nil
	}
	result.Span = tok.Span
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("each other", tok.GetCh(src))}
	result.Message = "Did you mean `each other`?"
	return &result
}
