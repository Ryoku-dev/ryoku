package text

import "unicode"

func IsCJK(c rune) bool {
	for _, b := range cjkBlocks {
		if c >= b[0] && c <= b[1] {
			return true
		}
	}
	return false
}

var cjkBlocks = [...][2]rune{{0x4e00, 0x9fff}, {0x3400, 0x4dbf}, {0x20000, 0x2a6df}, {0x2a700, 0x2b73f}, {0x2b740, 0x2b81f}, {0x2b820, 0x2ceaf}, {0x2ceb0, 0x2ebef}, {0x30000, 0x3134f}, {0x31350, 0x323af}, {0x2ebf0, 0x2ee5f}, {0x1100, 0x11ff}, {0xac00, 0xd7af}, {0xa960, 0xa97f}, {0xd7b0, 0xd7ff}, {0x3130, 0x318f}, {0x3000, 0x303f}, {0x31c0, 0x31ef}, {0x3300, 0x33ff}, {0xfe30, 0xfe4f}, {0xf900, 0xfaff}, {0x2f800, 0x2fa1f}, {0x2e80, 0x2eff}, {0x3200, 0x32ff}, {0x3040, 0x309f}}

// IsEmoji follows Harper's selected Unicode blocks, not the Unicode Emoji property.
func IsEmoji(c rune) bool {
	return c >= 0xfff0 && c <= 0xffff || c >= 0x1f600 && c <= 0x1f64f || c >= 0x2600 && c <= 0x26ff || c >= 0xfe00 && c <= 0xfe0f || c >= 0x1f900 && c <= 0x1f9ff
}
func IsEnglishLingual(c rune) bool {
	return !unicode.IsSpace(c) && !unicode.IsNumber(c) && !IsEmoji(c) && !IsPunctuation(c) && unicode.IsLetter(c) && !IsCJK(c) && unicode.In(c, unicode.Latin)
}
func IsPunctuation(c rune) bool { _, ok := PunctuationFromChar(c); return ok }
func IsVowel(c rune) bool {
	switch asciiLower(c) {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}
func NormalizedChar(c rune) rune {
	switch c {
	case '\u2018', '\u2019', '\u02bc', '\uff07':
		return '\''
	case '\u201c', '\u201d', '\uff02':
		return '"'
	case '\u2013', '\u2014', '\u2212', '\uff0d':
		return '-'
	}
	return c
}
