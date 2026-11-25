package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"slices"
)

// RepeatedWords looks for repetitions of words that are not homographs.
type RepeatedWords struct{ specialCases [][]rune }

func NewRepeatedWords() *RepeatedWords { return &RepeatedWords{specialCases: [][]rune{[]rune("this")}} }
func (r *RepeatedWords) isSpecialCase(chars []rune) bool {
	lower := text.ToLower(chars)
	for _, s := range r.specialCases {
		if slices.Equal(s, lower) {
			return true
		}
	}
	return false
}
func (*RepeatedWords) Description() string {
	return "This rule looks for repetitions of words that are not homographs."
}
func (r *RepeatedWords) Lint(d *doc.Document) []lint.Lint {
	var result []lint.Lint
	source := d.GetSource()
	for _, chunk := range d.IterChunks() {
		indices := doc.IterWordIndices(chunk)
		for i := 0; i+1 < len(indices); i++ {
			ia, ib := indices[i], indices[i+1]
			a, b := chunk[ia], chunk[ib]
			wa, wb := a.GetCh(source), b.GetCh(source)
			if informalLaughterIsInformalLaughter(wa) && informalLaughterIsInformalLaughter(wb) {
				continue
			}
			prev, pok := d.GetTokenOffset(ia, -1)
			next, nok := d.GetTokenOffset(ib, 1)
			if (pok && prev.Kind.IsHyphen()) || (nok && next.Kind.IsHyphen()) {
				continue
			}
			k := a.Kind
			if !(k.IsPreposition() || k.IsConjunction() || !k.IsLikelyHomograph() || r.isSpecialCase(wa) || k.IsAdverb() || k.IsDeterminer()) || !slices.Equal(text.ToLower(wa), text.ToLower(wb)) {
				continue
			}
			whitespace := true
			for _, t := range chunk[ia+1 : ib] {
				if !t.Kind.IsWhitespace() {
					whitespace = false
					break
				}
			}
			if !whitespace {
				continue
			}
			result = append(result, lint.Lint{Span: text.Span{Start: a.Span.Start, End: b.Span.End}, LintKind: lint.Repetition, Suggestions: []lint.Suggestion{lint.ReplaceWith(append([]rune(nil), wa...))}, Message: "Did you mean to repeat this word?", Priority: 128})
		}
	}
	return result
}
