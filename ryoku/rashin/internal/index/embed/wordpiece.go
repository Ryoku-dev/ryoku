package embed

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

func (m *Model) tokenizeIDs(text string) []int {
	var ids []int
	for _, word := range preTokenize(normalizeText(text)) {
		m.wordpiece(word, &ids)
	}
	return ids
}

func normalizeText(text string) string {
	var clean strings.Builder
	clean.Grow(len(text))
	for _, r := range text {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			clean.WriteByte(' ')
		case r == 0 || r == 0xfffd || isControl(r):
		case isWhitespace(r):
			clean.WriteByte(' ')
		case isCJK(r):
			clean.WriteByte(' ')
			clean.WriteRune(r)
			clean.WriteByte(' ')
		default:
			clean.WriteRune(r)
		}
	}
	var normalized strings.Builder
	normalized.Grow(clean.Len())
	for _, r := range norm.NFD.String(clean.String()) {
		if !unicode.Is(unicode.Mn, r) {
			normalized.WriteRune(unicode.ToLower(r))
		}
	}
	return normalized.String()
}

func preTokenize(text string) []string {
	var tokens []string
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}
	for _, r := range text {
		switch {
		case r == ' ' || isWhitespace(r):
			flush()
		case isPunctuation(r):
			flush()
			tokens = append(tokens, string(r))
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return tokens
}

func (m *Model) wordpiece(word string, ids *[]int) {
	runes := []rune(word)
	if len(runes) == 0 {
		return
	}
	if len(runes) > m.maxChars {
		*ids = append(*ids, m.unkID)
		return
	}
	pieces := make([]int, 0, 4)
	for start := 0; start < len(runes); {
		id, end := -1, len(runes)
		for end > start {
			piece := string(runes[start:end])
			if start > 0 {
				piece = "##" + piece
			}
			if value, ok := m.vocab[piece]; ok {
				id = value
				break
			}
			end--
		}
		if id < 0 {
			*ids = append(*ids, m.unkID)
			return
		}
		pieces = append(pieces, id)
		start = end
	}
	*ids = append(*ids, pieces...)
}

func isControl(r rune) bool {
	if r == '\t' || r == '\n' || r == '\r' {
		return false
	}
	return unicode.In(r, unicode.C)
}
func isWhitespace(r rune) bool { return r == ' ' || unicode.Is(unicode.Zs, r) }
func isPunctuation(r rune) bool {
	if (r >= 33 && r <= 47) || (r >= 58 && r <= 64) || (r >= 91 && r <= 96) || (r >= 123 && r <= 126) {
		return true
	}
	return unicode.IsPunct(r)
}
func isCJK(r rune) bool {
	switch {
	case r >= 0x4e00 && r <= 0x9fff, r >= 0x3400 && r <= 0x4dbf, r >= 0x20000 && r <= 0x2a6df, r >= 0x2a700 && r <= 0x2b73f, r >= 0x2b740 && r <= 0x2b81f, r >= 0x2b820 && r <= 0x2ceaf, r >= 0xf900 && r <= 0xfaff, r >= 0x2f800 && r <= 0x2fa1f:
		return true
	}
	return false
}
