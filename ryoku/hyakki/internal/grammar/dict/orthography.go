package dict

import (
	"ryoku-rashin/internal/grammar/text"
	"strings"
	"unicode"
)

const CASE_FLAGS OrthFlags = LOWERCASE | TITLECASE | ALLCAPS | LOWER_CAMEL | UPPER_CAMEL

func (f OrthFlags) CaseFlags() OrthFlags { return f & CASE_FLAGS }
func normalizeRune(c rune) rune          { return text.NormalizedChar(c) }
func normalized(w []rune) []rune         { return text.Normalized(w) }
func OrthFlagsFromLetters(letters []rune) OrthFlags {
	var flags OrthFlags
	allLower, allUpper, firstUpper, firstLower, sawUpper, sawLower, first := true, true, false, false, false, false, true
	count := 0
	for _, c := range letters {
		switch c {
		case ' ':
			flags |= MULTIWORD
			continue
		case '-':
			flags |= HYPHENATED
			continue
		}
		if normalizeRune(c) == '\'' {
			flags |= APOSTROPHE
			continue
		}
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') && !text.IsEnglishLingual(c) {
			continue
		}
		count++
		if unicode.IsLower(c) || unicode.In(c, unicode.Other_Lowercase) {
			allUpper = false
			if first {
				firstLower = true
			} else {
				sawLower = true
			}
		} else if unicode.IsUpper(c) || unicode.In(c, unicode.Other_Uppercase) {
			allLower = false
			if first {
				firstUpper = true
			} else {
				sawUpper = true
			}
		} else {
			firstUpper = false
			firstLower = false
		}
		first = false
	}
	if count > 0 {
		if allLower {
			flags |= LOWERCASE
		}
		if allUpper {
			flags |= ALLCAPS
		}
		if count > 1 && firstUpper && !sawUpper {
			flags |= TITLECASE
		}
		if firstLower && sawUpper {
			flags |= LOWER_CAMEL
		}
		if firstUpper && sawLower && sawUpper {
			flags |= UPPER_CAMEL
		}
	}
	if looksLikeRomanNumerals(letters) && isReallyRomanNumerals([]rune(strings.ToLower(string(letters)))) {
		flags |= ROMAN_NUMERALS
	}
	return flags
}
func looksLikeRomanNumerals(w []rune) bool {
	if len(w) == 0 {
		return false
	}
	upper := unicode.IsUpper(w[0])
	for _, c := range w {
		if !strings.ContainsRune("mdclxvi", unicode.ToLower(c)) || unicode.IsUpper(c) != upper {
			return false
		}
	}
	return true
}
func isReallyRomanNumerals(w []rune) bool {
	i := 0
	for i < len(w) && i < 4 && w[i] == 'm' {
		i++
	}
	group := func(one, five, ten rune) {
		if i >= len(w) {
			return
		}
		if w[i] == one {
			i++
			if i < len(w) && (w[i] == five || w[i] == ten) {
				i++
			} else {
				for n := 0; n < 2 && i < len(w) && w[i] == one; n++ {
					i++
				}
			}
		} else if w[i] == five {
			i++
			for n := 0; n < 3 && i < len(w) && w[i] == one; n++ {
				i++
			}
		}
	}
	group('c', 'd', 'm')
	group('x', 'l', 'c')
	group('i', 'v', 'x')
	return i == len(w)
}
