package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type AmountsFor struct{ expression expr.Expr }

func NewAmountsFor() *AmountsFor {
	return &AmountsFor{expression: expr.NewFirstMatchOf([]expr.Expr{expr.NewSequenceExprWordSet([]string{"that", "which", "it", "this"}).TWs().ThenFixedPhrase("amounts for"), expr.NewSequenceExprWordSet([]string{"they", "can", "could", "may", "might", "must", "should", "will", "would"}).TWs().ThenFixedPhrase("amount for")})}
}
func (r *AmountsFor) Expr() expr.Expr                  { return r.expression }
func (r *AmountsFor) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *AmountsFor) Description() string {
	return "Corrects `amounts for` to either `amounts to` or `accounts for`"
}
func (r *AmountsFor) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	all, ok := doc.Span(toks)
	if !ok || len(toks) < 5 {
		return nil
	}
	content := strings.ToLower(all.GetContentString(src))
	var replacements []string
	if strings.HasSuffix(content, "amounts for") {
		replacements = []string{"amounts to", "accounts for"}
	} else if strings.HasSuffix(content, "amount for") {
		replacements = []string{"amount to", "account for"}
	} else {
		return nil
	}
	span, _ := doc.Span(toks[2:5])
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.WordChoice
	l.Priority = 63
	l.Message = "`amounts for` is not idiomatic English. You probably meant `amounts to` or `accounts for`."
	l.Suggestions = lint.ToReplaceSuggestions(replacements, span.GetContent(src))
	return &l
}
