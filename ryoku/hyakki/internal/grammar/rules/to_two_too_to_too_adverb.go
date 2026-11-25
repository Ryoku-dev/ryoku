package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ToTooAdverb struct{ expression expr.Expr }

func NewToTooAdverb() *ToTooAdverb {
	return &ToTooAdverb{expression: expr.NewSequenceExprAco("to").TWs().ThenKindIsButIsNotExcept(doc.TokenKind.IsAdverb, doc.TokenKind.IsDeterminer, []string{"as", "only"}).ThenOptionalWhitespace().ThenAnyOf([]expr.Expr{expr.NewSequenceExpr().ThenKindIsButIsNotExcept(doc.TokenKind.IsPunctuation, func(doc.TokenKind) bool { return false }, []string{"`", "\"", "'", "“", "”", "‘", "’", "-", "–", "—"}), expr.NewAnchorEnd()})}
}
func (r *ToTooAdverb) Expr() expr.Expr { return r.expression }
func (r *ToTooAdverb) Description() string {
	return "Detects `to` before an adverb when it should be `too`."
}
func (r *ToTooAdverb) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ToTooAdverb) MatchToLint(tokens []doc.Token, source []rune) *lint.Lint {
	var target *doc.Token
	for i := range tokens {
		if text.EqStr(tokens[i].GetCh(source), "to") {
			target = &tokens[i]
			break
		}
	}
	if target == nil {
		return nil
	}
	result := lint.NewLint()
	result.Span = target.Span
	result.LintKind = lint.WordChoice
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("too", target.GetCh(source))}
	result.Message = "Use `too` here to mean ‘also’ or an excessive degree."
	return &result
}
func (r *ToTooAdverb) Unit() lint.DocumentIterator { return lint.Sentence{} }
