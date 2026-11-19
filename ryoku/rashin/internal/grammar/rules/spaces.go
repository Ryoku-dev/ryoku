package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
)

type Spaces struct{}

func NewSpaces() *Spaces            { return &Spaces{} }
func (*Spaces) Description() string { return "Words should be separated by at most one space." }
func (*Spaces) Lint(d *doc.Document) []lint.Lint {
	var result []lint.Lint
	for _, sentence := range doc.IterSentences(d.Tokens()) {
		for _, index := range doc.IterSpaceIndices(sentence) {
			if index == 0 {
				continue
			}
			space := sentence[index]
			count := space.Kind.Count
			if count > 1 {
				result = append(result, lint.Lint{Span: space.Span, LintKind: lint.Formatting, Suggestions: []lint.Suggestion{lint.ReplaceWith([]rune{' '})}, Message: fmt.Sprintf("There are %d spaces where there should be only one.", count), Priority: 15})
			}
		}
		n := len(sentence)
		if n >= 3 && sentence[n-3].Kind.IsWord() && sentence[n-2].Kind.IsSpace() && sentence[n-1].Kind.IsPunctuation() {
			result = append(result, lint.Lint{Span: sentence[n-2].Span, LintKind: lint.Formatting, Suggestions: []lint.Suggestion{lint.Remove()}, Message: "Unnecessary space at the end of the sentence.", Priority: 63})
		}
	}
	return result
}
