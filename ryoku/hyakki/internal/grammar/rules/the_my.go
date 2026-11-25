package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"unicode"
)

type TheMy struct{ expression expr.Expr }

func NewTheMy() *TheMy {
	the := expr.NewWord("the")
	poss := expr.NewWordSet([]string{"my", "your", "his", "her", "its", "our", "their"})
	return &TheMy{expression: expr.NewFirstMatchOf([]expr.Expr{expr.NewSequenceExprWith(the).ThenWhitespace().Then(poss), expr.NewSequenceExprWith(poss).ThenWhitespace().Then(the)})}
}
func (r *TheMy) Expr() expr.Expr                  { return r.expression }
func (r *TheMy) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *TheMy) Description() string {
	return "Flags the definite article used together with a possessive."
}
func (r *TheMy) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	first := toks[0].GetCh(src)
	second := toks[2].GetCh(src)
	poss := first
	if text.EqStr(first, "the") {
		poss = second
	}
	if unicode.IsUpper(second[0]) || text.EqStr(first, "her") {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Repetition, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCase(poss, span.GetContent(src)), lint.ReplaceWithMatchCaseStr("the", span.GetContent(src))}, Message: "Use either the definite article 'the' or the possessive. Using both together is ungrammatical in English.", Priority: 127}
}
