package text

import "unicode"

type Case uint8

const (
	Upper Case = iota
	Lower
)

func CaseFromChar(c rune) (Case, bool) {
	if unicode.IsUpper(c) || unicode.In(c, unicode.Other_Uppercase) {
		return Upper, true
	}
	if unicode.IsLower(c) || unicode.In(c, unicode.Other_Lowercase) {
		return Lower, true
	}
	return Lower, false
}
func (c Case) ApplyTo(ch rune) []rune {
	if c == Upper {
		if s, ok := uppercaseExpansion[ch]; ok {
			return []rune(s)
		}
		return []rune{unicode.ToUpper(ch)}
	}
	if ch == 'İ' {
		return []rune{'i', '\u0307'}
	}
	return []rune{unicode.ToLower(ch)}
}
func GetCasing(chars []rune) []Case {
	out := make([]Case, 0, len(chars))
	for _, c := range chars {
		if v, ok := CaseFromChar(c); ok {
			out = append(out, v)
		}
	}
	return out
}
func GetCasingUnfiltered(chars []rune) []*Case {
	out := make([]*Case, len(chars))
	cases := make([]Case, len(chars))
	for i, c := range chars {
		if v, ok := CaseFromChar(c); ok {
			cases[i] = v
			out[i] = &cases[i]
		}
	}
	return out
}

// CopyCasing skips uncased template characters and non-alphabetic target characters.
// The last template case is reused; an empty template lowercases the target.
func CopyCasing(template, target []rune) []rune {
	out := make([]rune, 0, len(target))
	pos := 0
	prev := Lower
	for _, c := range target {
		if unicode.IsLetter(c) || unicode.In(c, unicode.Nl, unicode.Other_Alphabetic) {
			for pos < len(template) {
				v, ok := CaseFromChar(template[pos])
				pos++
				if ok {
					prev = v
					break
				}
			}
		}
		out = appendCase(out, prev, c)
	}
	return out
}

func appendCase(out []rune, casing Case, c rune) []rune {
	if casing == Upper {
		if expansion, ok := uppercaseExpansion[c]; ok {
			for _, r := range expansion {
				out = append(out, r)
			}
			return out
		}
		return append(out, unicode.ToUpper(c))
	}
	if c == 'İ' {
		return append(out, 'i', '\u0307')
	}
	return append(out, unicode.ToLower(c))
}
