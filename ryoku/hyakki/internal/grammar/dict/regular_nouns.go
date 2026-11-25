package dict

import "ryoku-rashin/internal/grammar/text"

// GetPlurals gets all valid plural forms using regular patterns, validated
// against the dictionary. Only confirmed plural nouns are returned.
func GetPlurals(dict Dictionary, singular []rune) [][]rune {
	candidates := getPluralCandidates(singular)
	valid := candidates[:0]
	for _, candidate := range candidates {
		if md := dict.GetWordMetadata(candidate); md != nil && md.IsPluralNoun() {
			valid = append(valid, candidate)
		}
	}
	return valid
}

// GetSingulars gets all valid singular forms using regular patterns, validated
// against the dictionary. Only confirmed singular nouns are returned.
func GetSingulars(dict Dictionary, plural []rune) [][]rune {
	candidates := getSingularCandidates(plural)
	valid := candidates[:0]
	for _, candidate := range candidates {
		if md := dict.GetWordMetadata(candidate); md != nil && md.IsSingularNoun() {
			valid = append(valid, candidate)
		}
	}
	return valid
}

func getPluralCandidates(singular []rune) [][]rune {
	if len(singular) == 0 {
		return nil
	}
	candidates := make([][]rune, 0, 5)
	if len(singular) > 1 && text.EndsWithIgnoreASCIICaseChars(singular, []rune{'y'}) {
		candidates = append(candidates, nounWithSuffix(singular[:len(singular)-1], "ies"))
	}
	if text.EndsWithIgnoreASCIICaseChars(singular, []rune{'f', 'e'}) {
		candidates = append(candidates, nounWithSuffix(singular[:len(singular)-2], "ves"))
	}
	if text.EndsWithIgnoreASCIICaseChars(singular, []rune{'f'}) {
		candidates = append(candidates, nounWithSuffix(singular[:len(singular)-1], "ves"))
	}
	candidates = append(candidates, nounWithSuffix(singular, "s"))
	candidates = append(candidates, nounWithSuffix(singular, "es"))
	return candidates
}

func getSingularCandidates(plural []rune) [][]rune {
	if len(plural) == 0 {
		return nil
	}
	candidates := make([][]rune, 0, 5)
	if text.EndsWithIgnoreASCIICaseChars(plural, []rune{'i', 'e', 's'}) {
		candidates = append(candidates, nounWithSuffix(plural[:len(plural)-3], "y"))
	}
	if text.EndsWithIgnoreASCIICaseChars(plural, []rune{'v', 'e', 's'}) {
		candidates = append(candidates, nounWithSuffix(plural[:len(plural)-3], "fe"))
		candidates = append(candidates, nounWithSuffix(plural[:len(plural)-3], "f"))
	}
	if len(plural) > 2 && text.EndsWithIgnoreASCIICaseChars(plural, []rune{'e', 's'}) {
		candidates = append(candidates, nounWithSuffix(plural[:len(plural)-2], ""))
	}
	if text.EndsWithIgnoreASCIICaseChars(plural, []rune{'s'}) &&
		!text.EndsWithIgnoreASCIICaseChars(plural, []rune{'s', 's'}) {
		candidates = append(candidates, nounWithSuffix(plural[:len(plural)-1], ""))
	}
	return candidates
}

func nounWithSuffix(stem []rune, suffix string) []rune {
	word := make([]rune, len(stem)+len(suffix))
	copy(word, stem)
	for i, c := range suffix {
		word[len(stem)+i] = c
	}
	return word
}
