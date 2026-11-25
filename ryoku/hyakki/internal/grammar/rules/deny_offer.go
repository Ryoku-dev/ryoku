package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type DenyOffer struct{ expression expr.Expr }

func NewDenyOffer() *DenyOffer {
	return &DenyOffer{expression: expr.NewSequenceExprWordSet([]string{"deny", "denies", "denied", "denying"}).TWs().ThenOptional(expr.NewSequenceExpr().ThenDeterminer().TWs()).TSet([]string{"offer", "offers"})}
}
func (r *DenyOffer) Expr() expr.Expr                  { return r.expression }
func (r *DenyOffer) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *DenyOffer) Description() string {
	return "Corrects `deny` when used with `offer` to `decline` or `reject`."
}
func (r *DenyOffer) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	span := toks[0].Span
	chars := span.GetContent(src)
	if len(chars) == 0 {
		return nil
	}
	var corrections []string
	switch chars[len(chars)-1] {
	case 'y', 'Y':
		corrections = []string{"decline", "reject"}
	case 'd', 'D':
		corrections = []string{"declined", "rejected"}
	case 's', 'S':
		corrections = []string{"declines", "rejects"}
	case 'g', 'G':
		corrections = []string{"declining", "rejecting"}
	default:
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.WordChoice
	l.Suggestions = lint.ToReplaceSuggestions(corrections, chars)
	l.Message = "You would `deny` permission or a request, but `decline` or `reject` an offer."
	return &l
}
