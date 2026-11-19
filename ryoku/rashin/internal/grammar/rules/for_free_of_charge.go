package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type ForFreeOfCharge struct{ expression expr.Expr }

func NewForFreeOfCharge() *ForFreeOfCharge {
	return &ForFreeOfCharge{expression: expr.NewSequenceExprWordSeq([]string{"for", "free", "of", "charge"}).ThenAnyOf([]expr.Expr{expr.NewSequenceExpr().ThenKindAny([]expr.KindPredicate{doc.TokenKind.IsSentenceTerminator, doc.TokenKind.IsComma, doc.TokenKind.IsQuote}), expr.NewSequenceExprWhitespace().ThenKindAnyButNot([]expr.KindPredicate{doc.TokenKind.IsConjunction, doc.TokenKind.IsPreposition, doc.TokenKind.IsVerb}, doc.TokenKind.IsNoun)})}
}
func (r *ForFreeOfCharge) Expr() expr.Expr                  { return r.expression }
func (r *ForFreeOfCharge) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ForFreeOfCharge) Description() string {
	return "Corrects `for free of charge` to either `for free` or `free of charge`."
}
func (r *ForFreeOfCharge) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span, ok := doc.Span(t[:7])
	if !ok {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Redundancy
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("for free", s[span.Start:span.End]), lint.ReplaceWithMatchCaseStr("free of charge", s[span.Start:span.End])}
	result.Message = "Use only either `for free` or `free of charge`"
	return &result
}
