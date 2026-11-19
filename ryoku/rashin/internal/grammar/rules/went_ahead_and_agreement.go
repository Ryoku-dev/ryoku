package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type WentAheadAndAgreement struct {
	expression expr.Expr
	dictionary dict.Dictionary
}

func NewWentAheadAndAgreement(dictionary dict.Dictionary) *WentAheadAndAgreement {
	e := expr.NewSequenceExprWordSet([]string{"went", "gone"}).TWs().ThenWordSeq([]string{"ahead", "and"}).TWs().ThenKindWhere(func(k doc.TokenKind) bool {
		return k.IsVerbLemma() && !k.IsVerbPastForm() && !k.IsVerbSimplePastForm() && !k.IsVerbPastParticipleForm()
	})
	return &WentAheadAndAgreement{expression: e, dictionary: dictionary}
}
func (r *WentAheadAndAgreement) Expr() expr.Expr { return r.expression }
func (r *WentAheadAndAgreement) Description() string {
	return "Checks for `went ahead and` followed by a present tense verb, which should be past tense."
}
func (r *WentAheadAndAgreement) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *WentAheadAndAgreement) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	goTok := t[0]
	verb := t[len(t)-1]
	went := goTok.Kind.IsVerbSimplePastForm()
	gone := goTok.Kind.IsVerbPastParticipleForm()
	if !went && !gone {
		return nil
	}
	var pasts [][]rune
	irreg := dict.CuratedIrregularVerbs()
	var past string
	var ok bool
	if went {
		past, ok = irreg.GetPreteriteForLemma(verb.GetStr(s))
	} else {
		past, ok = irreg.GetPastParticipleForLemma(verb.GetStr(s))
	}
	if ok {
		pasts = append(pasts, []rune(past))
	}
	ch := verb.GetCh(s)
	candidates := [][]rune{append(append([]rune(nil), ch...), 'd'), append(append([]rune(nil), ch...), 'e', 'd')}
	if len(ch) > 0 && !text.IsVowel(ch[len(ch)-1]) {
		candidates = append(candidates, append(append([]rune(nil), ch...), ch[len(ch)-1], 'e', 'd'))
	}
	for _, candidate := range candidates {
		md := r.dictionary.GetWordMetadata(candidate)
		if md != nil && (md.IsVerbPastForm() || (went && md.IsVerbSimplePastForm()) || (gone && md.IsVerbPastParticipleForm())) {
			pasts = append(pasts, candidate)
		}
	}
	l := lint.NewLint()
	l.Span = verb.Span
	l.LintKind = lint.Agreement
	for _, past := range pasts {
		l.Suggestions = append(l.Suggestions, lint.ReplaceWithMatchCase(past, verb.GetCh(s)))
	}
	l.Message = "The tense of the verb after `and` should match the tense of `go`."
	return &l
}
