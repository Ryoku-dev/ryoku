package fs

import (
	"fmt"
	"math"
	"strings"
)

// Derived from oh-my-pi (MIT), crates/edit/src/fuzzy.rs and
// crates/edit/src/text.rs@c7fff67

const (
	fuzzyThreshold  = 0.95
	dominantMin     = 0.97
	fuzzyLineFloor  = 0.85
	dominantDelta   = 0.08
	fuzzyWork       = 300_000 // window × line comparisons before the similarity pass gives up
	similarityRunes = 256
	previewMatches  = 5
)

// replacement is the outcome of a find/replace on LF text.
type replacement struct {
	Text  string
	Count int
	Line  int    // first changed line, 1-based
	How   string // "" exact, else how old was matched
}

// replaceError is a failed find/replace, worded for the model.
type replaceError struct {
	Message string
	Lines   []int // every occurrence, for ambiguity
}

func (e *replaceError) Error() string { return e.Message }

// findReplace swaps old for new in content: exactly first, then with copied
// read prefixes removed, then line by line ignoring whitespace and quote
// style, then by close similarity. It never guesses between two places.
func findReplace(content, old, new string, all bool, shown string) (replacement, error) {
	old = strings.ReplaceAll(old, "\r\n", "\n")
	new = strings.ReplaceAll(new, "\r\n", "\n")
	if old == "" {
		return replacement{}, &replaceError{Message: "old is empty. Copy the text to replace from the file into old, or use write to create a file."}
	}
	if old == new {
		return replacement{}, &replaceError{Message: "old and new are the same, so nothing would change. Put the new text in new."}
	}
	if r, err, ok := exactReplace(content, old, new, all, shown); ok {
		return r, err
	}
	if so, ok := stripRowNumbers(old); ok {
		sn := new
		if s, ok := stripRowNumbers(new); ok {
			sn = s
		}
		if r, err, ok := exactReplace(content, so, sn, all, shown); ok {
			if err == nil {
				r.How = "after removing the N: prefixes copied from read"
			}
			return r, err
		}
		old, new = so, sn
	}
	return lineReplace(content, old, new, all, shown)
}

// exactReplace reports ok when old occurs at least once.
func exactReplace(content, old, new string, all bool, shown string) (replacement, error, bool) {
	n := strings.Count(content, old)
	switch {
	case n == 0:
		return replacement{}, nil, false
	case n > 1 && !all:
		var at []int
		from := 0
		for range n {
			i := strings.Index(content[from:], old)
			at = append(at, strings.Count(content[:from+i], "\n")+1)
			from += i + len(old)
		}
		return replacement{}, ambiguous(shown, at, lines(content)), true
	}
	first := strings.Count(content[:strings.Index(content, old)], "\n") + 1
	if all {
		return replacement{Text: strings.ReplaceAll(content, old, new), Count: n, Line: first}, nil, true
	}
	return replacement{Text: strings.Replace(content, old, new, 1), Count: 1, Line: first}, nil, true
}

func ambiguous(shown string, at []int, rows []string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "old matches %d places in %s, at lines %s. Add the lines around the one you mean to old so it matches once, or set all to true to change every one.", len(at), shown, joinInts(at, previewMatches))
	for i, n := range at {
		if i == previewMatches {
			break
		}
		if n-1 < len(rows) {
			row, _ := clipLine(strings.TrimSpace(rows[n-1]), 80)
			fmt.Fprintf(&b, "\n  %d: %s", n, row)
		}
	}
	return &replaceError{Message: b.String(), Lines: at}
}

func joinInts(ns []int, most int) string {
	var parts []string
	for i, n := range ns {
		if i == most {
			parts = append(parts, fmt.Sprintf("and %d more", len(ns)-most))
			break
		}
		parts = append(parts, fmt.Sprint(n))
	}
	return strings.Join(parts, ", ")
}

// lineReplace matches old as a run of whole lines, first by normalised
// equality, then by similarity at or above fuzzyThreshold.
func lineReplace(content, old, new string, all bool, shown string) (replacement, error) {
	rows := strings.Split(content, "\n")
	if strings.HasSuffix(old, "\n") {
		old = strings.TrimSuffix(old, "\n")
		new = strings.TrimSuffix(new, "\n")
	}
	want := strings.Split(old, "\n")
	notFound := &replaceError{Message: fmt.Sprintf("old was not found in %s. Read the file again and copy old exactly from it, including spaces and line breaks.", shown)}
	if len(want) > len(rows) {
		return replacement{}, notFound
	}
	normWant := normalizeRows(want)
	var hits []int
	for s := 0; s+len(want) <= len(rows); s++ {
		if equalRows(normalizeRows(rows[s:s+len(want)]), normWant) {
			hits = append(hits, s)
			s += len(want) - 1
		}
	}
	if len(hits) > 1 && !all {
		at := make([]int, len(hits))
		for i, h := range hits {
			at[i] = h + 1
		}
		return replacement{}, ambiguous(shown, at, rows)
	}
	how := "ignoring whitespace and quote style"
	if len(hits) == 0 {
		best, score, ok := similar(rows, want, normWant)
		if !ok {
			if best < 0 {
				return replacement{}, notFound
			}
			return replacement{}, closest(shown, want, rows[best:best+len(want)], best+1, score)
		}
		hits = []int{best}
		how = fmt.Sprintf("by close similarity (%d%%)", int(math.Round(score*100)))
	}
	newRows := strings.Split(new, "\n")
	out := make([]string, 0, len(rows)+len(hits)*(len(newRows)-len(want)))
	prev := 0
	for _, h := range hits {
		out = append(out, rows[prev:h]...)
		adjusted := adjustIndent(want, rows[h:h+len(want)], newRows)
		if new != "" || len(newRows) > 1 {
			out = append(out, adjusted...)
		}
		prev = h + len(want)
	}
	out = append(out, rows[prev:]...)
	return replacement{Text: strings.Join(out, "\n"), Count: len(hits), Line: hits[0] + 1, How: how}, nil
}

// similar considers multiline typos only when no whole-line match exists.
func similar(rows, want, normWant []string) (best int, score float64, ok bool) {
	windows := len(rows) - len(want) + 1
	if len(want) < 2 || windows <= 0 || windows*len(want) > fuzzyWork {
		return -1, 0, false
	}
	best, score = -1, -1.0
	second := -1.0
	above := 0
	for s := range windows {
		got := normalizeRows(rows[s : s+len(want)])
		total := 0.0
		everyLine := true
		for i := range want {
			lineScore := similarity(normWant[i], got[i])
			if lineScore < fuzzyLineFloor {
				everyLine = false
			}
			total += lineScore
		}
		v := total / float64(len(want))
		if everyLine && v >= fuzzyThreshold {
			above++
		}
		if !everyLine {
			continue
		}
		if v > score {
			second, score, best = score, v, s
		} else if v > second {
			second = v
		}
	}
	if score >= fuzzyThreshold && (above == 1 || (score >= dominantMin && score-second >= dominantDelta)) {
		return best, score, true
	}
	return best, score, false
}

func closest(shown string, want, got []string, line int, score float64) error {
	o, g := want[0], got[0]
	for i := range want {
		if strings.TrimSpace(want[i]) != strings.TrimSpace(got[i]) {
			o, g = want[i], got[i]
			break
		}
	}
	o, _ = clipLine(o, 160)
	g, _ = clipLine(g, 160)
	return &replaceError{Message: fmt.Sprintf(
		"old was not found in %s. The closest text (%d%% alike) starts at line %d; the first line that differs is:\n  old:  %s\n  file: %s\nRead the file again and copy old exactly from it.",
		shown, int(math.Round(score*100)), line, o, g)}
}

func normalizeRows(rows []string) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = normalizeRow(r)
	}
	return out
}

// normalizeRow folds what models get wrong when they copy a line: spacing,
// curly quotes and dash styles.
func normalizeRow(line string) string {
	var b strings.Builder
	space := false
	for _, c := range strings.TrimSpace(line) {
		switch c {
		case '\u201C', '\u201D', '\u201E', '\u201F', '\u00AB', '\u00BB':
			c = '"'
		case '\u2018', '\u2019', '\u201A', '\u201B', '`', '\u00B4':
			c = '\''
		case '\u2010', '\u2011', '\u2012', '\u2013', '\u2014', '\u2212':
			c = '-'
		case '\t':
			c = ' '
		}
		if c == ' ' {
			if space {
				continue
			}
			space = true
		} else {
			space = false
		}
		b.WriteRune(c)
	}
	return b.String()
}

func equalRows(a, b []string) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// similarity is 1 minus the edit distance over the longer length.
func similarity(a, b string) float64 {
	if a == b {
		return 1
	}
	if a == "" || b == "" {
		return 0
	}
	ar, br := []rune(a), []rune(b)
	if len(ar) > similarityRunes {
		ar = ar[:similarityRunes]
	}
	if len(br) > similarityRunes {
		br = br[:similarityRunes]
	}
	longest := max(len(ar), len(br))
	return 1 - float64(levenshtein(ar, br))/float64(longest)
}

func levenshtein(a, b []rune) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

// adjustIndent shifts new by the indentation the file's lines carry over
// old's, when that shift is the same on every line and the indent character
// agrees; otherwise new is used as written.
func adjustIndent(old, actual, new []string) []string {
	if equalRows(old, actual) {
		return new
	}
	delta, char, ok := indentDelta(old, actual)
	if !ok || delta == 0 {
		return new
	}
	out := make([]string, len(new))
	for i, line := range new {
		if strings.TrimSpace(line) == "" {
			out[i] = line
			continue
		}
		if delta > 0 {
			out[i] = strings.Repeat(string(char), delta) + line
			continue
		}
		cut := 0
		for cut < -delta && cut < len(line) && line[cut] == char {
			cut++
		}
		out[i] = line[cut:]
	}
	return out
}

func indentDelta(old, actual []string) (int, byte, bool) {
	delta, set := 0, false
	var char byte
	for i := range old {
		if strings.TrimSpace(old[i]) == "" || strings.TrimSpace(actual[i]) == "" {
			continue
		}
		oi, oc := leading(old[i])
		ai, ac := leading(actual[i])
		if oc != 0 && ac != 0 && oc != ac {
			return 0, 0, false
		}
		if ac != 0 {
			char = ac
		} else if oc != 0 && char == 0 {
			char = oc
		}
		d := ai - oi
		if set && d != delta {
			return 0, 0, false
		}
		delta, set = d, true
	}
	if char == 0 {
		char = ' '
	}
	return delta, char, set
}

// leading counts a line's indentation and its character; 0 when the line
// has none or mixes tabs and spaces.
func leading(line string) (int, byte) {
	n := 0
	var c byte
	for n < len(line) && (line[n] == ' ' || line[n] == '\t') {
		if c == 0 {
			c = line[n]
		} else if line[n] != c {
			c = 0xff
		}
		n++
	}
	if c == 0xff {
		return n, 0
	}
	return n, c
}
