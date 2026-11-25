// Package rank holds the scoring shared by the code index and memory: word
// tokens, BM25, int8 vectors with their cosine, and reciprocal rank fusion.
package rank

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxTerms is how many distinct terms one document keeps for BM25: its most
// frequent, then longest.
const MaxTerms = 16

// Tokens splits text into lower-case words, breaking identifiers at
// underscores, dots, colons, case changes and digit runs.
func Tokens(text string) []string {
	var tokens []string
	var word []rune
	flush := func() {
		if len(word) == 0 {
			return
		}
		parts := SplitIdentifier(string(word))
		tokens = append(tokens, parts...)
		word = word[:0]
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			word = append(word, r)
		} else {
			flush()
		}
	}
	flush()
	return tokens
}

// SplitIdentifier breaks one identifier into its lower-case parts.
func SplitIdentifier(value string) []string {
	value = strings.Trim(value, "_")
	if value == "" {
		return nil
	}
	var out []string
	for _, section := range strings.FieldsFunc(value, func(r rune) bool { return r == '_' || r == '.' || r == ':' }) {
		runes := []rune(section)
		start := 0
		for i := 1; i < len(runes); i++ {
			boundary := (unicode.IsLower(runes[i-1]) && unicode.IsUpper(runes[i])) || (i+1 < len(runes) && unicode.IsUpper(runes[i-1]) && unicode.IsUpper(runes[i]) && unicode.IsLower(runes[i+1])) || (unicode.IsDigit(runes[i-1]) != unicode.IsDigit(runes[i]))
			if boundary {
				if i > start {
					out = append(out, strings.ToLower(string(runes[start:i])))
				}
				start = i
			}
		}
		if start < len(runes) {
			out = append(out, strings.ToLower(string(runes[start:])))
		}
	}
	return out
}

// Frequencies counts the terms of text worth indexing: words longer than one
// letter and digits, at most MaxTerms of them.
func Frequencies(text string) map[string]int {
	all := map[string]int{}
	for _, token := range Tokens(text) {
		first, _ := utf8.DecodeRuneInString(token)
		if len(token) > 1 || unicode.IsDigit(first) {
			all[token]++
		}
	}
	if len(all) <= MaxTerms {
		return all
	}
	terms := make([]string, 0, len(all))
	for term := range all {
		terms = append(terms, term)
	}
	sort.Slice(terms, func(left, right int) bool {
		if all[terms[left]] != all[terms[right]] {
			return all[terms[left]] > all[terms[right]]
		}
		if len(terms[left]) != len(terms[right]) {
			return len(terms[left]) > len(terms[right])
		}
		return terms[left] < terms[right]
	})
	out := make(map[string]int, MaxTerms)
	for _, term := range terms[:MaxTerms] {
		out[term] = all[term]
	}
	return out
}

// TermCount is a document's length in terms.
func TermCount(terms map[string]int) int {
	n := 0
	for _, count := range terms {
		n += count
	}
	return n
}
