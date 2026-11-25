package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

// HelpedPast catches double-tensing errors when a past-tense verb incorrectly follows "helped".
type HelpedPast struct {
	expression expr.Expr
	dictionary dict.Dictionary
}

func NewHelpedPast(dictionary dict.Dictionary) *HelpedPast {
	return &HelpedPast{dictionary: dictionary, expression: expr.ButNot(expr.NewSequenceExprAco("helped").TWs().ThenKindWhere(func(k doc.TokenKind) bool {
		return (k.IsVerbSimplePastForm() || k.IsVerbPastForm()) && !k.IsVerbLemma()
	}), expr.NewSequenceExprAnything().TAny().TSet([]string{"did", "got"}))}
}
func (r *HelpedPast) Expr() expr.Expr                  { return r.expression }
func (r *HelpedPast) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *HelpedPast) Description() string {
	return "Corrects past forms of verbs to their base form, when used after \"helped\"."
}
func (r *HelpedPast) keepSuggestionIfLemma(suggestions *[][]rune, candidate []rune) {
	if md := r.dictionary.GetWordMetadata(candidate); md != nil && md.IsVerbLemma() {
		*suggestions = append(*suggestions, append([]rune(nil), candidate...))
	}
}
func (r *HelpedPast) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	span := toks[len(toks)-1].Span
	ch := span.GetContent(src)
	var suggestions [][]rune
	if text.EndsWithIgnoreASCIICaseStr(ch, "d") {
		withoutD := ch[:len(ch)-1]
		if text.EndsWithIgnoreASCIICaseStr(withoutD, "e") {
			withoutEd := withoutD[:len(withoutD)-1]
			r.keepSuggestionIfLemma(&suggestions, withoutEd)
			if text.EndsWithIgnoreASCIICaseStr(withoutEd, "i") {
				candidate := append(append([]rune(nil), withoutEd[:len(withoutEd)-1]...), 'y')
				r.keepSuggestionIfLemma(&suggestions, candidate)
			}
			if len(withoutEd) > 0 && !text.IsVowel(withoutEd[len(withoutEd)-1]) {
				r.keepSuggestionIfLemma(&suggestions, withoutEd[:len(withoutEd)-1])
			}
		}
		r.keepSuggestionIfLemma(&suggestions, withoutD)
	}
	if lemma, ok := dict.CuratedIrregularVerbs().GetLemmaForPreterite(string(ch)); ok {
		suggestions = append(suggestions, []rune(lemma))
	}
	if len(suggestions) == 0 {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Grammar
	l.Message = "Use the base form of the verb after \"helped\"."
	for _, s := range suggestions {
		l.Suggestions = append(l.Suggestions, lint.ReplaceWithMatchCase(s, ch))
	}
	return &l
}
