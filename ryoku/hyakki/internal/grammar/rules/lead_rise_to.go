package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type LeadRiseTo struct{ expression expr.Expr }

func NewLeadRiseTo() *LeadRiseTo {
	return &LeadRiseTo{expression: expr.NewSequenceExprWordSet([]string{"lead", "led", "leads", "leading"}).TWs().TAco("rise").TWs().TAco("to")}
}
func (r *LeadRiseTo) Expr() expr.Expr                  { return r.expression }
func (r *LeadRiseTo) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *LeadRiseTo) Description() string              { return "Corrects `leads rise to` to `gives rise to`." }
func (r *LeadRiseTo) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	span := toks[0].Span
	var values []string
	switch string(span.GetContent(src)) {
	case "lead":
		values = []string{"give", "gave", "given"}
	case "led":
		values = []string{"gave", "given"}
	case "leads":
		values = []string{"gives"}
	case "leading":
		values = []string{"giving"}
	default:
		return nil
	}
	out := lint.NewLint()
	out.Span = span
	out.LintKind = lint.Usage
	out.Suggestions = lint.ToReplaceSuggestions(values, span.GetContent(src))
	out.Message = "The correct idiom is `give rise to`."
	return &out
}
