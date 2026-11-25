package dict

import (
	"unicode"

	"ryoku-rashin/internal/grammar/text"
)

type InitialSound uint8

const (
	Vowel InitialSound = iota
	Consonant
	Either
)

// StartsWithVowel checks whether a word begins with a vowel sound. The boolean
// is false if word is empty. Produced through trial and error, this matches
// 99.71% and 99.77% of vowels and non-vowels in the Carnegie-Mellon University
// word-to-pronunciation dataset.
func StartsWithVowel(word []rune, dialect Dialect) (InitialSound, bool) {
	if len(word) == 0 {
		return 0, false
	}
	if articleWordEquals(word, "LED") || articleWordEquals(word, "SQL") || articleWordEquals(word, "URL") {
		return Either, true
	}
	if len(word) >= 2 {
		firstCase, firstOK := text.CaseFromChar(word[0])
		secondCase, secondOK := text.CaseFromChar(word[1])
		if firstOK && secondOK && secondCase == text.Upper {
			for i, c := range word {
				casing, ok := text.CaseFromChar(c)
				if !ok || casing != firstCase {
					word = word[:i]
					break
				}
			}
		}
	}
	likelyInitialism := true
	for _, c := range word {
		casing, ok := text.CaseFromChar(c)
		if articleIsAlphabetic(c) && (!ok || casing != text.Upper) {
			likelyInitialism = false
			break
		}
	}
	if len(word) == 1 || (likelyInitialism && !isLikelyAcronym(word)) {
		c := word[0]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		switch c {
		case 'A', 'E', 'F', 'H', 'I', 'L', 'M', 'N', 'O', 'R', 'S', 'X':
			return Vowel, true
		default:
			return Consonant, true
		}
	}
	word = toLowerWord(word)
	if articleWordPrefix(word, "ubi") {
		return Either, true
	}
	if len(word) >= 3 && word[0] == 'm' && word[1] == 'p' && word[2] >= '0' && word[2] <= '9' {
		return Vowel, true
	}
	if articleWordEquals(word, "npm") || articleWordPrefix(word, "eule") {
		return Vowel, true
	}
	if articleWordPrefix(word, "uk") || articleWordPrefix(word, "ude") ||
		articleWordPrefix(word, "euph") || articleWordPrefix(word, "eug") ||
		articleWordPrefix(word, "eul") || articleWordPrefix(word, "euc") ||
		articleWordPrefix(word, "one") || articleWordEquals(word, "once") {
		return Consonant, true
	}
	if articleWordPrefix(word, "hour") || articleWordPrefix(word, "unin") ||
		articleWordPrefix(word, "unim") || articleWordPrefix(word, "una") ||
		articleWordPrefix(word, "unu") || articleWordPrefix(word, "urb") || articleWordPrefix(word, "int") {
		return Vowel, true
	}
	if articleWordPrefix(word, "herb") && (dialect == American || dialect == Canadian) {
		return Vowel, true
	}
	if len(word) >= 3 && word[0] == 'u' && (word[1] == 'n' || word[1] == 's') &&
		(word[2] == 'i' || word[2] == 'a' || word[2] == 'u') {
		return Consonant, true
	}
	if articleWordPrefix(word, "un") || articleWordPrefix(word, "urg") || articleWordPrefix(word, "utt") {
		return Vowel, true
	}
	if len(word) >= 2 && word[0] == 'u' && (word[1] == 't' || word[1] == 'r' || word[1] == 'n') ||
		articleWordPrefix(word, "eur") || articleWordPrefix(word, "uw") || articleWordPrefix(word, "use") {
		return Consonant, true
	}
	if len(word) >= 5 && articleWordPrefix(word, "one") &&
		(word[3] == 'a' || word[3] == 'e' || word[3] == 'i' || word[3] == 'u') &&
		(word[4] == 'l' || word[4] == 'd') {
		return Vowel, true
	}
	if len(word) >= 4 && articleWordPrefix(word, "one") &&
		(word[3] == 'a' || word[3] == 'e' || word[3] == 'i' || word[3] == 'u' || word[3] == '-' || word[3] == 's') {
		return Consonant, true
	}
	if articleWordEquals(word, "sos") || articleWordPrefix(word, "rz") ||
		articleWordPrefix(word, "ng") || articleWordPrefix(word, "nv") ||
		articleWordEquals(word, "xbox") || articleWordPrefix(word, "heir") ||
		articleWordPrefix(word, "honor") || articleWordPrefix(word, "hones") {
		return Vowel, true
	}
	if len(word) >= 3 && word[0] == 'j' && (word[1] == 'u' || word[1] == 'o') && word[2] == 'n' ||
		len(word) >= 4 && articleWordPrefix(word, "jur") && (word[3] == 'a' || word[3] == 'i' || word[3] == 'o') {
		return Consonant, true
	}
	if len(word) >= 2 && word[0] == 'x' &&
		(word[1] == '-' || word[1] == '\'' || word[1] == '.' || word[1] == 'o' || word[1] == 's') {
		return Vowel, true
	}
	if text.IsVowel(word[0]) {
		return Vowel, true
	}
	return Consonant, true
}

func toLowerWord(word []rune) []rune {
	for _, c := range word {
		if casing, ok := text.CaseFromChar(c); ok && casing == text.Upper {
			return text.ToLower(word)
		}
	}
	return word
}

func wordContainsFalsePositiveSequence(word []rune) bool {
	for i := 1; i < len(word); i++ {
		if word[i-1] == 'V' && word[i] == 'C' {
			return true
		}
	}
	return false
}

func isLikelyAcronym(word []rune) bool {
	const minLen = 3
	if len(word) < minLen || wordContainsFalsePositiveSequence(word) {
		return false
	}
	for _, c := range word[:minLen] {
		if !articleIsAlphabetic(c) {
			return false
		}
	}
	return !text.IsVowel(word[0]) && text.IsVowel(word[1])
}

func articleIsAlphabetic(c rune) bool {
	return unicode.IsLetter(c) || unicode.Is(unicode.Nl, c) || unicode.Is(unicode.Other_Alphabetic, c)
}

func articleWordPrefix(word []rune, prefix string) bool {
	if len(word) < len(prefix) {
		return false
	}
	for i := range len(prefix) {
		if word[i] != rune(prefix[i]) {
			return false
		}
	}
	return true
}

func articleWordEquals(word []rune, value string) bool {
	return len(word) == len(value) && articleWordPrefix(word, value)
}
