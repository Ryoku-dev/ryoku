package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type SummarySummery struct{ expression expr.Expr }

func NewSummarySummery() *SummarySummery {
	return &SummarySummery{expr.NewFirstMatchOf([]expr.Expr{expr.NewSequenceExprAco("summary").TWs().TSet([]string{"dress", "dresses"}), expr.NewSequenceExprWordSet([]string{"executive", "financial"}).TWs().TAco("summery"), expr.NewSequenceExprAco("summery").TWsH().TSet([]string{"about", "above", "below", "by", "for", "in", "into", "of", "to", "with", "box", "boxes", "div", "control", "controls", "element", "elements", "field", "fields", "log", "logs", "object", "objects", "page", "pages", "row", "rows", "screen", "screens", "section", "sections", "tab", "tabs"})})}
}
func (r *SummarySummery) Expr() expr.Expr                  { return r.expression }
func (r *SummarySummery) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*SummarySummery) Description() string {
	return "Checks for the common confusion between `summary` and `summery`."
}
func (r *SummarySummery) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	w := lint.FindTheOnlyTokenMatching(t, s, func(w *doc.Token, s []rune) bool {
		return text.EqAnyIgnoreASCIICaseStr(w.GetCh(s), []string{"summary", "summery"})
	})
	if w == nil {
		return nil
	}
	correction := append([]rune(nil), w.GetCh(s)...)
	if len(correction) <= 4 {
		return nil
	}
	switch correction[4] {
	case 'a':
		correction[4] = 'e'
	case 'A':
		correction[4] = 'E'
	case 'e':
		correction[4] = 'a'
	case 'E':
		correction[4] = 'A'
	default:
		return nil
	}
	return &lint.Lint{Span: w.Span, LintKind: lint.Spelling, Suggestions: []lint.Suggestion{lint.ReplaceWith(correction)}, Message: "Did you mean `summery` (relating to summer) or `summary` (brief statement)?", Priority: 127}
}
