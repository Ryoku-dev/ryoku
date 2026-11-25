package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type DidPast struct {
	expression expr.Expr
	dictionary dict.Dictionary
}

func NewDidPast(dictionary dict.Dictionary) *DidPast {
	return &DidPast{dictionary: dictionary, expression: expr.NewSequenceExprLongestOf([]expr.Expr{expr.AsExpr(expr.NewWordSet([]string{"did", "didn't", "didnt"})), expr.NewFixedPhraseFromPhrase("did not")}).ThenOptional(expr.NewSequenceExpr().TWs().ThenSubjectPronoun()).TWs().ThenKindWhere(func(k doc.TokenKind) bool {
		return (k.IsVerbSimplePastForm() || k.IsVerbPastForm()) && !k.IsVerbLemma()
	})}
}
func (r *DidPast) Expr() expr.Expr                  { return r.expression }
func (r *DidPast) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *DidPast) Description() string {
	return "Corrects past forms of verbs to their base form, when used together with \"did\"."
}
func (r *DidPast) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	tok := t[len(t)-1]
	v := tok.GetCh(s)
	lower := strings.ToLower(string(v))
	var suggestions []lint.Suggestion
	keep := func(candidate []rune) {
		if md := r.dictionary.GetWordMetadata(candidate); md != nil && md.IsVerbLemma() {
			suggestions = append(suggestions, lint.ReplaceWithMatchCase(candidate, v))
		}
	}
	if strings.HasSuffix(lower, "d") {
		withoutD := v[:len(v)-1]
		if strings.HasSuffix(strings.ToLower(string(withoutD)), "e") {
			withoutEd := withoutD[:len(withoutD)-1]
			keep(withoutEd)
			if strings.HasSuffix(strings.ToLower(string(withoutEd)), "i") {
				candidate := append([]rune(nil), withoutEd[:len(withoutEd)-1]...)
				candidate = append(candidate, 'y')
				keep(candidate)
			}
			if len(withoutEd) > 0 && !strings.ContainsRune("aeiouAEIOU", withoutEd[len(withoutEd)-1]) {
				keep(withoutEd[:len(withoutEd)-1])
			}
		}
		keep(withoutD)
	}
	if lemma, ok := dict.CuratedIrregularVerbs().GetLemmaForPreterite(string(v)); ok {
		suggestions = append(suggestions, lint.ReplaceWithMatchCaseStr(lemma, v))
	}
	if len(suggestions) == 0 {
		return nil
	}
	result := lint.NewLint()
	result.Span = tok.Span
	result.LintKind = lint.Grammar
	result.Suggestions = suggestions
	result.Message = "Use the base form of the verb with \"did\"."
	return &result
}
