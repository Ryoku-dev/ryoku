package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type WorthToDo struct {
	expression expr.Expr
	dictionary dict.Dictionary
}

func NewWorthToDo(dictionary dict.Dictionary) *WorthToDo {
	return &WorthToDo{expression: expr.NewSequenceExprWordSeq([]string{"worth", "to"}).TWs().ThenVerbLemma(), dictionary: dictionary}
}
func (r *WorthToDo) Expr() expr.Expr { return r.expression }
func (r *WorthToDo) Description() string {
	return "Corrects `worth to` + a verb to `worth` + the gerund of the verb."
}
func (r *WorthToDo) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *WorthToDo) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) < 3 {
		return nil
	}
	last := t[len(t)-1]
	span := text.Span{Start: t[len(t)-3].Span.Start, End: last.Span.End}
	ch := last.GetCh(s)
	word := string(ch)
	gerunds := []string{}
	if r.dictionary.ContainsWordStr(word + "ing") {
		gerunds = append(gerunds, word+"ing")
	}
	if text.EndsWithIgnoreASCIICaseStr(ch, "e") {
		g := string(ch[:len(ch)-1]) + "ing"
		if r.dictionary.ContainsWordStr(g) {
			gerunds = append(gerunds, g)
		}
	}
	if len(ch) > 0 && !text.IsVowel(ch[len(ch)-1]) {
		g := word + string(ch[len(ch)-1]) + "ing"
		if r.dictionary.ContainsWordStr(g) {
			gerunds = append(gerunds, g)
		}
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Grammar
	result.Suggestions = lint.ToReplaceSuggestions(gerunds, span.GetContent(s))
	result.Message = "Use the `gerund` of the verb, the form that ends in `-ing`"
	return &result
}
