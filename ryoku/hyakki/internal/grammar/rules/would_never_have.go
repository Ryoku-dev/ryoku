package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type WouldNeverHave struct{ expression expr.Expr }

func NewWouldNeverHave() *WouldNeverHave {
	phrases := []string{"could have never", "could never have", "could've never", "couldve never", "would have never", "would never have", "would've never", "wouldve never"}
	es := make([]expr.Expr, 0, len(phrases))
	for _, p := range phrases {
		es = append(es, expr.NewFixedPhraseFromPhrase(p))
	}
	e := expr.NewSequenceExprAnyOf(es).ThenWhitespace().ThenVerb()
	return &WouldNeverHave{expression: e}
}
func (r *WouldNeverHave) Expr() expr.Expr { return r.expression }
func (r *WouldNeverHave) Description() string {
	return "Corrects `would/could have never` to `never would/could have`."
}
func (r *WouldNeverHave) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *WouldNeverHave) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	first := t[0]
	word := strings.ToLower(first.GetStr(s))
	modal := ""
	if strings.HasPrefix(word, "could") {
		modal = "could"
	} else if strings.HasPrefix(word, "would") {
		modal = "would"
	} else {
		return nil
	}
	tail := " have"
	if strings.HasSuffix(word, "ve") {
		tail = "'ve"
	}
	phrasing := "never " + modal + tail
	span, _ := doc.Span(t[:len(t)-2])
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Style
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(phrasing, first.GetCh(s))}
	l.Message = "For a more standard style, consider using `" + phrasing + "`."
	return &l
}
