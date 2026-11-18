package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
	"unicode"
)

type AnA struct{ dialect dict.Dialect }

func NewAnA(dialect dict.Dialect) *AnA { return &AnA{dialect: dialect} }
func (r *AnA) Description() string {
	return "A rule that looks for incorrect indefinite articles. For example, `this is an mule` would be flagged as incorrect."
}
func (r *AnA) Lint(document *doc.Document) []lint.Lint {
	var lints []lint.Lint
	source := document.GetSource()
	for _, chunk := range doc.IterChunks(document.Tokens()) {
		indices := doc.IterWordIndices(chunk)
		for i := 1; i < len(indices); i++ {
			firstIdx, secondIdx := indices[i-1], indices[i]
			if len(doc.IterUnlintables(chunk[firstIdx:secondIdx])) > 0 || len(doc.IterWordLikes(chunk[firstIdx+1:secondIdx])) > 0 {
				continue
			}
			first, second := chunk[firstIdx], chunk[secondIdx]
			firstChars, secondChars := first.GetCh(source), second.GetCh(source)
			for j, c := range secondChars {
				if !unicode.IsLetter(c) && !unicode.IsNumber(c) {
					secondChars = secondChars[:j]
					break
				}
			}
			if string(firstChars) == "A" {
				if secondIdx == firstIdx+2 && chunk[firstIdx+1].Kind.IsHyphen() && len(secondChars) > 0 && secondChars[0] >= 'B' && secondChars[0] <= 'F' {
					continue
				}
				if prev, ok := document.GetNextWordFromOffset(firstIdx, -1); ok {
					s := strings.ToLower(prev.GetStr(source))
					if s == "a" || s == "an" {
						continue
					}
				}
			}
			s := string(firstChars)
			if s != "a" && s != "A" && s != "an" && s != "An" {
				continue
			}
			isA := s == "a" || s == "A"
			if meta, ok := second.Kind.AsWord(); ok && meta != nil && len(secondChars) > 1 && meta.OrthInfo.Contains(dict.LOWERCASE) && !meta.OrthInfo.Contains(dict.ALLCAPS) && !meta.OrthInfo.Contains(dict.ROMAN_NUMERALS) {
				caps := true
				for _, c := range secondChars {
					if unicode.IsLetter(c) && !unicode.IsUpper(c) {
						caps = false
						break
					}
				}
				if caps {
					secondChars = []rune(strings.ToLower(string(secondChars)))
				}
			}
			sound, ok := dict.StartsWithVowel(secondChars, r.dialect)
			if !ok {
				panic("No empty word tokens")
			}
			if sound == dict.Either {
				return lints
			}
			shouldA := sound == dict.Consonant
			if isA != shouldA {
				replacement := "a"
				if isA {
					replacement = "an"
				}
				lints = append(lints, lint.Lint{Span: first.Span, LintKind: lint.Miscellaneous, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr(replacement, firstChars)}, Message: "Incorrect indefinite article.", Priority: 31})
			}
		}
	}
	return lints
}
