package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type VeryLess struct{ expression expr.Expr }

func NewVeryLess() *VeryLess {
	return &VeryLess{expression: expr.ButNot(expr.NewSequenceExpr().ThenDegreeAdverb().TWs().TAco("less"), expr.NewWordSet([]string{"comparably", "indeed", "notably", "really", "so"}))}
}
func (r *VeryLess) Expr() expr.Expr { return r.expression }
func (r *VeryLess) Description() string {
	return "Corrects adverbs of degree (`too`, `very`, etc.) used with `less` mostly in the writing of native German speakers.`"
}
func (r *VeryLess) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *VeryLess) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) != 3 {
		return nil
	}
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	degree, ws, less := toks[0].GetStr(src), toks[1].GetStr(src), toks[2].GetStr(src)
	var corrections []string
	if text.EqStr(toks[0].GetCh(src), "very") {
		for _, v := range []string{"a lot", "far", "much"} {
			corrections = append(corrections, v+ws+less)
		}
	} else if text.EqStr(toks[0].GetCh(src), "quite") {
		for _, v := range []string{"quite a bit", "quite a lot"} {
			corrections = append(corrections, v+ws+less)
		}
	} else {
		for _, v := range []string{"few", "little"} {
			corrections = append(corrections, degree+ws+v)
		}
	}
	suggestions := make([]lint.Suggestion, 0, len(corrections))
	for _, v := range corrections {
		suggestions = append(suggestions, lint.ReplaceWithMatchCaseStr(v, src[span.Start:span.End]))
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Usage
	result.Message = "English does not use words like ‘too’ or ‘very’ to modify ‘less’. • For comparisons, use ‘far less’ or ‘much less’, etc. • If describing a small amoung or quantity, use ‘very little’ (furniture, information, traffic, etc.) or ‘very few’ (items, people, things, etc.)."
	result.Suggestions = suggestions
	return &result
}
