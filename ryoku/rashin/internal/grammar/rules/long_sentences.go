package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

// LongSentences detects and warns that a sentence is too long.
type LongSentences struct{}

func NewLongSentences() *LongSentences { return &LongSentences{} }
func (*LongSentences) Description() string {
	return "This rule looks for run-on sentences, which can make your work harder to grok."
}
func (*LongSentences) Lint(d *doc.Document) []lint.Lint {
	var output []lint.Lint
	for _, sentence := range doc.IterSentences(d.Tokens()) {
		words := doc.IterWords(sentence)
		if len(words) > 40 {
			output = append(output, lint.Lint{Span: text.Span{Start: words[0].Span.Start, End: sentence[len(sentence)-1].Span.End}, LintKind: lint.Readability, Message: fmt.Sprintf("This sentence is %d words long.", len(words)), Priority: 127})
		}
	}
	return output
}
