package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ToTooDegreeWords struct{ expression expr.Expr }

func NewToTooDegreeWords() *ToTooDegreeWords {
	return &ToTooDegreeWords{expression: expr.NewSequenceExprAco("to").TWs().ThenWordSet([]string{"many", "much", "few"}).ThenAnyOf([]expr.Expr{expr.NewSequenceExpr().ThenKindIsButIsNotExcept(doc.TokenKind.IsPunctuation, func(doc.TokenKind) bool { return false }, []string{"`", "\"", "'", "“", "”", "‘", "’", "-", "–", "—"}), expr.NewAnchorEnd()})}
}
func (r *ToTooDegreeWords) Expr() expr.Expr { return r.expression }
func (r *ToTooDegreeWords) Description() string {
	return "Detects `to` used before degree words like `many`, `much`, or `few`."
}
func (r *ToTooDegreeWords) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ToTooDegreeWords) MatchToLint(tokens []doc.Token, source []rune) *lint.Lint {
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
