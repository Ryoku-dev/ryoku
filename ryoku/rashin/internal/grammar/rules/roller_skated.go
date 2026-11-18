package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

// RollerSkated suggests hyphenating the past tense of `roller-skate`.
type RollerSkated struct{ expression *expr.ExprMap[int] }

func NewRollerSkated() *RollerSkated {
	m := expr.NewExprMap[int]()
	m.Insert(expr.NewSequenceExpr().ThenKindIsButIsNot(doc.TokenKind.IsWord, doc.TokenKind.IsDeterminer).ThenWhitespace().ThenSeq(rollerSkatedPair()), 2)
	m.Insert(expr.NewSequenceExpr().ThenPunctuation().ThenWhitespace().ThenSeq(rollerSkatedPair()), 2)
	m.Insert(expr.NewSequenceExpr().ThenPunctuation().ThenSeq(rollerSkatedPair()), 1)
	m.Insert(expr.NewSequenceExprWith(expr.NewAnchorStart()).ThenSeq(rollerSkatedPair()), 0)
	return &RollerSkated{expression: m}
}
func (r *RollerSkated) Expr() expr.Expr                  { return r.expression }
func (r *RollerSkated) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *RollerSkated) Description() string {
	return "Encourages hyphenating the past tense of `roller-skate`."
}
func (r *RollerSkated) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	i, ok := r.expression.Lookup(0, t, s)
	if !ok || i+2 >= len(t) {
		return nil
	}
	span, ok := doc.Span(t[i : i+3])
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Punctuation, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("roller-skated", s[span.Start:span.End])}, Message: "Hyphenate this verb as `roller-skated`.", Priority: 40}
}
func rollerSkatedPair() *expr.SequenceExpr {
	return expr.NewSequenceExpr().TAco("roller").TWs().TAco("skated")
}
