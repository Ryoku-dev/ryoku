package reflex

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Catalog resolves a spoken app name to an installed app: its catalog
// name, the command it runs and a score 0..1, where 1 is an exact name or
// alias. ok is false when nothing is close enough or two different apps
// are too close to tell apart; a catalog decides that with Best.
type Catalog interface {
	Resolve(spoken string) (name, exec string, score float64, ok bool)
}

// plain lower-cases s and keeps only its letters and digits, so "Fire
// Fox", "firefox" and "fire-fox" compare equal.
func plain(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// sounds folds spellings that are said alike onto one spelling: "ph" and
// "f", a hard "c" and "k", "x" and "ks", "z" and "s", "y" and "i", the
// long vowels written two ways, and doubled letters. The result is only
// ever compared with another folded word.
var sounds = strings.NewReplacer(
	"ph", "f", "gh", "", "ck", "k", "qu", "kw", "x", "ks",
	"ce", "se", "ci", "si", "cy", "si", "c", "k", "z", "s",
	"ee", "i", "ea", "i", "ie", "i", "oo", "u", "ou", "u", "y", "i",
)

func folded(s string) string {
	s = sounds.Replace(plain(s))
	var b strings.Builder
	var last rune
	for _, r := range s {
		if r != last {
			b.WriteRune(r)
		}
		last = r
	}
	return b.String()
}

// distance is the Levenshtein edit distance between a and b in runes.
func distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

func similarity(a, b string) float64 {
	n := max(utf8.RuneCountInString(a), utf8.RuneCountInString(b))
	if n == 0 {
		return 0
	}
	return 1 - float64(distance(a, b))/float64(n)
}

// Soundslike scores how alike two names sound, 0..1. Names that are the
// same once case, spaces and punctuation go score 1; otherwise the score
// is the better of the spelled and the said-alike comparison, each one
// less the edit distance over the longer length.
func Soundslike(a, b string) float64 {
	pa, pb := plain(a), plain(b)
	if pa == "" || pb == "" {
		return 0
	}
	if pa == pb {
		return 1
	}
	return max(similarity(pa, pb), similarity(folded(a), folded(b)))
}

// family is the first word of a name: "files" and "files viewer" are one
// family.
func family(name string) string {
	f := strings.Fields(strings.ToLower(name))
	if len(f) == 0 {
		return ""
	}
	return plain(f[0])
}

// Best picks the name in names that spoken sounds most like. It returns
// the index and score, or ok false when the best score is under the spec's
// reflex.fuzzyMin, or when a name from another family scores within
// reflex.fuzzyMargin of it. Names of one family (same first word) never
// crowd each other out: among them the higher score wins, then the shorter
// name. An exact match always wins.
func Best(spoken string, names []string) (index int, score float64, ok bool) {
	lim := limits()
	index = -1
	scores := make([]float64, len(names))
	for i, n := range names {
		scores[i] = Soundslike(spoken, n)
		if index < 0 || scores[i] > scores[index] ||
			(scores[i] == scores[index] && len(plain(n)) < len(plain(names[index]))) {
			index = i
		}
	}
	if index < 0 {
		return -1, 0, false
	}
	score = scores[index]
	if score >= 1 {
		return index, score, true
	}
	if score < lim.FuzzyMin {
		return -1, score, false
	}
	top := family(names[index])
	for i, n := range names {
		if i != index && family(n) != top && score-scores[i] < lim.FuzzyMargin {
			return -1, score, false
		}
	}
	return index, score, true
}
