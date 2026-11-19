package text

import (
	"unicode"
	"unicode/utf8"
)

func ToString(chars []rune) string { return string(chars) }

// ToLower and Normalized preserve the input slice when no character changes.
func ToLower(chars []rune) []rune {
	var out []rune
	for i, c := range chars {
		lower := unicode.ToLower(c)
		if out == nil && (lower != c || c == 'İ') {
			out = make([]rune, 0, len(chars))
			out = append(out, chars[:i]...)
		}
		if out != nil {
			if c == 'İ' {
				out = append(out, 'i', '\u0307')
			} else {
				out = append(out, lower)
			}
		}
	}
	if out == nil {
		return chars
	}
	return out
}
func Normalized(chars []rune) []rune {
	for i, c := range chars {
		if NormalizedChar(c) != c {
			out := append([]rune(nil), chars...)
			for j := i; j < len(out); j++ {
				out[j] = NormalizedChar(out[j])
			}
			return out
		}
	}
	return chars
}
func asciiLower(c rune) rune {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}
func EqCh(chars, other []rune) bool {
	for _, c := range other {
		if c >= 'A' && c <= 'Z' {
			panic("EqCh requires a lowercase ASCII right-hand side")
		}
	}
	if len(chars) != len(other) {
		return false
	}
	for i, c := range chars {
		if asciiLower(c) != other[i] {
			return false
		}
	}
	return true
}
func EqChLenient(chars, other []rune) bool {
	if len(chars) != len(other) {
		return false
	}
	for i, c := range chars {
		if asciiLower(c) != asciiLower(other[i]) {
			return false
		}
	}
	return true
}
func EqStr(chars []rune, other string) bool {
	for _, c := range other {
		if c >= 'A' && c <= 'Z' {
			panic("EqStr requires a lowercase ASCII right-hand side")
		}
	}
	i := 0
	for _, c := range other {
		if i >= len(chars) || asciiLower(chars[i]) != c {
			return false
		}
		i++
	}
	return i == len(chars)
}
func EqStrLenient(chars []rune, other string) bool {
	i := 0
	for _, c := range other {
		if i >= len(chars) || asciiLower(chars[i]) != asciiLower(c) {
			return false
		}
		i++
	}
	return i == len(chars)
}
func EqAnyIgnoreASCIICaseStr(chars []rune, others []string) bool {
	for _, v := range others {
		if EqStr(chars, v) {
			return true
		}
	}
	return false
}
func EqAnyIgnoreASCIICaseChars(chars []rune, others [][]rune) bool {
	for _, v := range others {
		if EqCh(chars, v) {
			return true
		}
	}
	return false
}
func StartsWithIgnoreASCIICaseStr(chars []rune, prefix string) bool {
	i := 0
	for _, c := range prefix {
		if i >= len(chars) || asciiLower(chars[i]) != c {
			return false
		}
		i++
	}
	return true
}
func StartsWithAnyIgnoreASCIICaseStr(chars []rune, prefixes []string) bool {
	for _, v := range prefixes {
		if StartsWithIgnoreASCIICaseStr(chars, v) {
			return true
		}
	}
	return false
}
func EndsWithIgnoreASCIICaseChars(chars, suffix []rune) bool {
	if len(chars) < len(suffix) {
		return false
	}
	offset := len(chars) - len(suffix)
	for i, c := range suffix {
		if asciiLower(chars[offset+i]) != c {
			return false
		}
	}
	return true
}
func EndsWithIgnoreASCIICaseStr(chars []rune, suffix string) bool {
	n := utf8.RuneCountInString(suffix)
	return len(chars) >= n && StartsWithIgnoreASCIICaseStr(chars[len(chars)-n:], suffix)
}
func EndsWithAnyIgnoreASCIICaseChars(chars []rune, suffixes [][]rune) bool {
	for _, v := range suffixes {
		if EndsWithIgnoreASCIICaseChars(chars, v) {
			return true
		}
	}
	return false
}
func ContainsVowel(chars []rune) bool {
	for _, c := range chars {
		if IsVowel(c) {
			return true
		}
	}
	return false
}
func StripPrefixIgnoreASCIICaseChars(chars, prefix []rune) ([]rune, bool) {
	if len(chars) < len(prefix) || !EqChLenient(chars[:len(prefix)], prefix) {
		return nil, false
	}
	return chars[len(prefix):], true
}

// EqIgnoreASCIICaseChars and EqIgnoreASCIICaseStr accept a lowercase right-hand side.
func EqIgnoreASCIICaseChars(chars, other []rune) bool      { return EqCh(chars, other) }
func EqIgnoreASCIICaseStr(chars []rune, other string) bool { return EqStr(chars, other) }
