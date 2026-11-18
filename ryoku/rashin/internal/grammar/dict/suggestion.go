package dict

import (
	"sort"
	"strings"
)

func asciiLower(c rune) rune {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}
func IsOuMisspelling(a, b []rune) bool { return singleExtraAfter(a, b, 'o', 'u') }
func IsLlMisspelling(a, b []rune) bool { return singleExtraAfter(a, b, 'l', 'l') }
func singleExtraAfter(a, b []rune, lead, extra rune) bool {
	if abs(len(a)-len(b)) != 1 {
		return false
	}
	i, j := 0, 0
	type letter struct {
		value   rune
		present bool
	}
	next := func(w []rune, index *int) letter {
		if *index == len(w) {
			return letter{}
		}
		c := asciiLower(w[*index])
		*index++
		return letter{c, true}
	}
	for {
		ca, cb := next(a, &i), next(b, &j)
		if ca == (letter{lead, true}) && cb == (letter{lead, true}) {
			x, y := next(a, &i), next(b, &j)
			if x != y {
				if x == (letter{extra, true}) {
					x = next(a, &i)
				} else if y == (letter{extra, true}) {
					y = next(b, &j)
				}
				if x != y {
					return false
				}
			}
		} else if !ca.present && !cb.present {
			return true
		} else if ca != cb {
			return false
		}
	}
}
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
func IsCkszMisspelling(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	if asciiLower(a[0]) != asciiLower(b[0]) {
		return false
	}
	found := false
	for i, x := range a {
		y := asciiLower(b[i])
		x = asciiLower(x)
		if x == y {
			continue
		}
		if !((x == 's' && y == 'z') || (x == 'z' && y == 's') || (x == 's' && y == 'c') || (x == 'c' && y == 's') || (x == 'k' && y == 'c') || (x == 'c' && y == 'k')) || found {
			return false
		}
		found = true
	}
	return found
}
func IsErMisspelling(a, b []rune) bool {
	if len(a) != len(b) || len(a) <= 4 {
		return false
	}
	n := len(a)
	a1, a2, b1, b2 := asciiLower(a[n-2]), asciiLower(a[n-1]), asciiLower(b[n-2]), asciiLower(b[n-1])
	if !((a1 == 'r' && a2 == 'e' && b1 == 'e' && b2 == 'r') || (a1 == 'e' && a2 == 'r' && b1 == 'r' && b2 == 'e')) {
		return false
	}
	for i := range n - 2 {
		if asciiLower(a[i]) != asciiLower(b[i]) {
			return false
		}
	}
	return true
}
func IsThHMissing(a, b []rune) bool {
	found := false
	for _, c := range a {
		if asciiLower(c) == 't' {
			found = true
			break
		}
	}
	if !found {
		return false
	}
	for i := 1; i < len(b); i++ {
		if asciiLower(b[i-1]) == 't' && asciiLower(b[i]) == 'h' {
			return true
		}
	}
	return false
}
func IsAyEyMisspelling(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	found := false
	for i := 0; i < len(a); i++ {
		x, y := asciiLower(a[i]), asciiLower(b[i])
		if x == y {
			continue
		}
		if !found && ((x == 'a' && y == 'e') || (x == 'e' && y == 'a')) && i+1 < len(a) && asciiLower(a[i+1]) == 'y' && asciiLower(b[i+1]) == 'y' {
			found = true
			i++
			continue
		}
		return false
	}
	return found
}
func IsEiIeMisspelling(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	found := false
	for i := 0; i < len(a); i++ {
		x, y := asciiLower(a[i]), asciiLower(b[i])
		if x == y {
			continue
		}
		if !found && ((x == 'e' && y == 'i') || (x == 'i' && y == 'e')) && i+1 < len(a) && asciiLower(a[i+1]) == y && asciiLower(b[i+1]) == x {
			found = true
			i++
			continue
		}
		return false
	}
	return found
}
func containsVowel(a []rune) bool {
	for _, c := range a {
		if strings.ContainsRune("aeiou", asciiLower(c)) {
			return true
		}
	}
	return false
}
func scoreSuggestion(w []rune, s FuzzyMatchResult) int {
	if len(w) == 0 || len(s.Word) == 0 {
		return int(^uint32(0) >> 1)
	}
	v := s.Word
	score := int(s.EditDistance) * 10
	if asciiLower(w[0]) == asciiLower(v[0]) {
		score -= 10
	}
	if w[len(w)-1] == 's' && v[len(v)-1] == 's' {
		score -= 5
	}
	apostropheDiff := func(longer, shorter []rune) bool {
		for i, c := range longer {
			if c == '\'' || c == '’' {
				return len(longer)-1 == len(shorter) && i <= len(shorter) && string(longer[:i]) == string(shorter[:i]) && string(longer[i+1:]) == string(shorter[i:])
			}
		}
		return false
	}
	diff := len(w) - len(v)
	if diff == 1 && apostropheDiff(w, v) {
		score -= 8
	} else if diff == -1 && s.Metadata.IsApostrophized() && apostropheDiff(v, w) {
		score -= 8
	}
	if s.Metadata.Common && len(s.Metadata.DerivedFrom) == 0 {
		score -= 4
	}
	if strings.Count(string(v), "'") == 1 {
		score -= 5
	}
	if IsThHMissing(w, v) {
		score -= 6
	}
	if !containsVowel(w) && !containsVowel(v) {
		score += 10
	}
	if s.EditDistance == 1 && (IsCkszMisspelling(w, v) || IsOuMisspelling(w, v) || IsLlMisspelling(w, v) || IsAyEyMisspelling(w, v) || IsThHMissing(w, v)) {
		score -= 6
	}
	if s.EditDistance <= 2 {
		if IsEiIeMisspelling(w, v) {
			score -= 11
		}
		if IsErMisspelling(w, v) {
			score -= 15
		}
	}
	return score
}
func SuggestCorrectSpelling(w []rune, resultLimit int, maxEditDist uint8, dictionary Dictionary) [][]rune {
	matches := dictionary.FuzzyMatch(w, maxEditDist, resultLimit)
	sort.SliceStable(matches, func(i, j int) bool { return scoreSuggestion(w, matches[i]) < scoreSuggestion(w, matches[j]) })
	out := make([][]rune, 0, len(matches))
	for _, v := range matches {
		out = append(out, v.Word)
	}
	return out
}
func SuggestCorrectSpellingStr(w string, resultLimit int, maxEditDist uint8, dictionary Dictionary) []string {
	matches := SuggestCorrectSpelling([]rune(w), resultLimit, maxEditDist, dictionary)
	out := make([]string, 0, len(matches))
	for _, v := range matches {
		out = append(out, string(v))
	}
	return out
}
