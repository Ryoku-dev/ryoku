package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type NoContractionWithVerb struct{ expression expr.Expr }

func NewNoContractionWithVerb() *NoContractionWithVerb {
	single := expr.NewSequenceExpr().ThenKindIsButIsntAnyOf(doc.TokenKind.IsVerb, []expr.KindPredicate{doc.TokenKind.IsNoun, doc.TokenKind.IsAdjective, doc.TokenKind.IsVerbProgressiveForm})
	following := expr.NewSequenceExpr().ThenVerb().TWs().ThenKindAny([]expr.KindPredicate{doc.TokenKind.IsDeterminer, doc.TokenKind.IsPronoun, doc.TokenKind.IsConjunction})
	return &NoContractionWithVerb{expression: expr.NewSequenceExprWordSet([]string{"lets", "let"}).TWs().Then(expr.NewLongestMatchOf([]expr.Expr{single, following}))}
}
func (r *NoContractionWithVerb) Expr() expr.Expr { return r.expression }
func (r *NoContractionWithVerb) Description() string {
	return "Checks for `lets` meaning `permits` when the context is about suggesting an action."
}
func (r *NoContractionWithVerb) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *NoContractionWithVerb) MatchToLint(tokens []doc.Token, source []rune) *lint.Lint {
	if len(tokens) < 3 {
		return nil
	}
	if tokens[0].GetStr(source) == "let" && tokens[2].GetStr(source) == "go" {
		return nil
	}
	target := tokens[0]
	result := lint.NewLint()
	result.Span = target.Span
	result.LintKind = lint.WordChoice
	result.Priority = 31
	result.Suggestions = lint.ToReplaceSuggestions([]string{"let's", "let us"}, target.GetCh(source))
	result.Message = "To suggest an action, use 'let's' or 'let us'."
	return &result
}
