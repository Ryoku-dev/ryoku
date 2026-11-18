package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
	"unicode"
)

type SentenceCapitalization struct{ dictionary dict.Dictionary }

func NewSentenceCapitalization(dictionary dict.Dictionary) *SentenceCapitalization {
	return &SentenceCapitalization{dictionary: dictionary}
}

// Lint checks that the first word of each sentence is capitalized.
func (r *SentenceCapitalization) Lint(d *doc.Document) []lint.Lint {
	var out []lint.Lint
	for _, paragraph := range d.IterParagraphs() {
		sentences := doc.IterSentences(paragraph)
		if len(sentences) == 1 {
			long := false
			for _, chunk := range doc.IterChunks(sentences[0]) {
				if len(doc.IterWords(chunk)) > 5 {
					long = true
					break
				}
			}
			if !long {
				continue
			}
		}
	sentenceLoop:
		for _, sentence := range sentences {
			if !sentenceCapitalizationIsFullSentence(sentence) {
				continue
			}
			first, ok := doc.FirstNonWhitespace(sentence)
			if !ok || !first.Kind.IsWord() {
				continue
			}
			chars := first.GetCh(d.GetSource())
			if len(chars) == 0 || !unicode.IsLetter(chars[0]) || unicode.IsUpper(chars[0]) {
				continue
			}
			for _, canonical := range r.dictionary.GetCorrectCapitalizationsOf(chars) {
				if first.Kind.IsProperNoun() {
					continue sentenceLoop
				}
				for _, c := range canonical[1:] {
					if unicode.IsSpace(c) || c == '-' || c == '\'' {
						break
					}
					if unicode.IsUpper(c) {
						continue sentenceLoop
					}
				}
			}
			replacement := append([]rune(nil), chars...)
			if replacement[0] >= 'a' && replacement[0] <= 'z' {
				replacement[0] -= 'a' - 'A'
			}
			out = append(out, lint.Lint{Span: first.Span, LintKind: lint.Capitalization, Suggestions: []lint.Suggestion{lint.ReplaceWith(replacement)}, Priority: 31, Message: "This sentence does not start with a capital letter"})
		}
	}
	return out
}
func (r *SentenceCapitalization) Description() string {
	return "The opening word of a sentence should almost always be capitalized."
}
func sentenceCapitalizationIsFullSentence(toks []doc.Token) bool {
	nominal, verb := false, false
	for _, tok := range toks {
		if tok.Kind.IsWord() && tok.Kind.Metadata != nil {
			nominal = nominal || tok.Kind.Metadata.IsNominal()
			verb = verb || tok.Kind.Metadata.IsVerb()
		}
	}
	return nominal && verb
}
