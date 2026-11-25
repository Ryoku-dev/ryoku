package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
	"unicode"
)

type Months struct{ expression expr.Expr }

func NewMonths() *Months {
	ambiguous := expr.NewWordSet([]string{"march", "may", "august"})
	only := expr.NewWordSet()
	for _, m := range monthsAllMonths {
		if !ambiguous.Contains(m) {
			only.Add(m)
		}
	}
	before := expr.NewWordSet([]string{"every", "by", "during", "in", "last", "next", "of", "until"})
	number := expr.NewSequenceExpr().ThenKindWhere(func(k doc.TokenKind) bool {
		if k.Kind != doc.Number {
			return false
		}
		v := k.Number.Value
		return v >= 1500 && v < 2501 || v >= 1 && v < 32
	})
	return &Months{expression: expr.NewSequenceExprWith(expr.NewFirstMatchOf([]expr.Expr{only, expr.NewSequenceExprWith(before).ThenWhitespace().Then(ambiguous), expr.NewSequenceExprWith(ambiguous).ThenWhitespace().Then(number)}))}
}
func (r *Months) Expr() expr.Expr                  { return r.expression }
func (r *Months) Description() string              { return "Detects months written with a lowercase first letter." }
func (r *Months) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *Months) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	var month *doc.Token
	for i := range t {
		name := strings.ToLower(t[i].GetStr(s))
		for _, m := range monthsAllMonths {
			if name == m {
				month = &t[i]
				break
			}
		}
		if month != nil {
			break
		}
	}
	if month == nil {
		return nil
	}
	ch := month.GetCh(s)
	if len(ch) == 0 || unicode.IsUpper(ch[0]) {
		return nil
	}
	replacement := append([]rune(nil), ch...)
	if replacement[0] >= 'a' && replacement[0] <= 'z' {
		replacement[0] -= 'a' - 'A'
	}
	return &lint.Lint{Span: month.Span, LintKind: lint.Miscellaneous, Suggestions: []lint.Suggestion{lint.ReplaceWith(replacement)}, Message: "Months should be written with a capital letter.", Priority: 126}
}

var monthsAllMonths = []string{"january", "february", "march", "april", "may", "june", "july", "august", "september", "october", "november", "december"}
