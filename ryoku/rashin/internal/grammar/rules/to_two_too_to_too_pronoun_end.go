package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ToTooPronounEnd struct{ expression expr.Expr }

func NewToTooPronounEnd() *ToTooPronounEnd {
	return &ToTooPronounEnd{expression: expr.NewSequenceExprAnyOf([]expr.Expr{expr.NewSequenceExprWith(expr.NewAnchorStart()), expr.NewSequenceExpr().ThenKindIsButIsNotExcept(doc.TokenKind.IsPunctuation, func(doc.TokenKind) bool { return false }, []string{"`", "\"", "'", "“", "”", "‘", "’"}).ThenOptionalWhitespace()}).ThenPronoun().TWs().TAco("to").ThenAnyOf([]expr.Expr{expr.NewSequenceExpr().ThenKindIsButIsNotExcept(doc.TokenKind.IsPunctuation, func(doc.TokenKind) bool { return false }, []string{"`", "\"", "'", "“", "”", "‘", "’"}), expr.NewAnchorEnd()})}
}
func (r *ToTooPronounEnd) Expr() expr.Expr { return r.expression }
func (r *ToTooPronounEnd) Description() string {
	return "Detects `to` after a pronoun at clause end (e.g., `Me to!`)."
}
func (r *ToTooPronounEnd) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ToTooPronounEnd) MatchToLint(tokens []doc.Token, source []rune) *lint.Lint {
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
