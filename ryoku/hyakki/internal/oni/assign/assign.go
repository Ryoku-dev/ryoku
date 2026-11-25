// Package assign holds the rules of assignment mode: whether what the
// perched window shows now is a new problem or the same one with an answer
// typed into it, the shape a solve must come back in, whether two answers
// agree, the solved views kept to go back to, and the day's spend.
//
// It is pure apart from the day's spend, which a Store keeps in one small
// file: no capture, no model, no goroutine. The daemon takes the pictures,
// reads their words and sends the requests.
package assign

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"ryoku-rashin/internal/oni/screenask"
)

// View holds a coarse row-major fingerprint and positioned OCR words.
// It never holds the picture.
type View struct {
	Gray  []uint8
	Words []screenask.Word
}

// GateConfig is the gate's numbers. Still is the largest mean absolute
// difference, 0–255, of two fingerprints that needs critical evidence to
// count as a new problem; Gone the fraction of the earlier view's words
// that, once missing from a larger change, makes a new problem; Critical
// how many earlier numbers and operators may change before it does.
type GateConfig struct {
	Still    float64
	Gone     float64
	Critical int
}

// Verdict is the gate's answer and the step that gave it.
type Verdict struct {
	New bool
	// By is "pixels", "words" or "critical".
	By string
}

// NewProblem says whether cur shows a new problem after prev. Identical
// fingerprints stay the same despite OCR variation. Small visual changes
// need a changed problem label or more missing critical tokens than allowed;
// larger changes also consider the share of words that went away. Added
// words are an answer being typed, never a new problem. A clock ticking
// changes one token and is not new.
func (c GateConfig) NewProblem(prev, cur View) Verdict {
	if len(prev.Gray) == 0 || len(prev.Gray) != len(cur.Gray) {
		return Verdict{New: true, By: "pixels"}
	}
	delta := meanDelta(prev.Gray, cur.Gray)
	if delta == 0 {
		return Verdict{By: "pixels"}
	}
	still := delta <= c.Still
	if !still {
		before, now := bag(prev.Words), bag(cur.Words)
		if len(before) == 0 {
			return Verdict{New: len(now) > 0, By: "words"}
		}
		gone, total := 0, 0
		for w, n := range before {
			total += n
			if left := n - now[w]; left > 0 {
				gone += left
			}
		}
		if float64(gone) >= c.Gone*float64(total) {
			return Verdict{New: true, By: "words"}
		}
	}
	if label(prev.Words) != label(cur.Words) {
		return Verdict{New: true, By: "critical"}
	}
	if changedCritical(prev.Words, cur.Words) > c.Critical {
		return Verdict{New: true, By: "critical"}
	}
	if still {
		return Verdict{By: "pixels"}
	}
	return Verdict{By: "words"}
}

func meanDelta(a, b []uint8) float64 {
	sum := 0
	for i := range a {
		d := int(a[i]) - int(b[i])
		if d < 0 {
			d = -d
		}
		sum += d
	}
	return float64(sum) / float64(len(a))
}

// bag counts each normalised word.
func bag(words []screenask.Word) map[string]int {
	out := map[string]int{}
	for _, w := range words {
		for _, t := range tokens(w.Text) {
			out[t]++
		}
	}
	return out
}

func tokens(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(",;:!?\"“”'‘’()[]{}", r)
	})
}

type criticalRow struct {
	top, bottom float64
	tokens      map[string]int
}

func criticalRows(words []screenask.Word) []criticalRow {
	var rows []criticalRow
	for _, word := range words {
		top, bottom := word.Box.Y, word.Box.Y+word.Box.H
		index := -1
		for i := range rows {
			if min(bottom, rows[i].bottom) > max(top, rows[i].top) {
				index = i
				break
			}
		}
		if index < 0 {
			index = len(rows)
			rows = append(rows, criticalRow{top: top, bottom: bottom})
		}
		row := &rows[index]
		row.top, row.bottom = min(row.top, top), max(row.bottom, bottom)
		row.tokens = addCritical(row.tokens, word.Text)
	}
	return rows
}

func changedCritical(before, after []screenask.Word) int {
	previous, current := criticalRows(before), criticalRows(after)
	now := make(map[string]int)
	changed := 0
	for _, row := range previous {
		if len(row.tokens) == 0 {
			continue
		}
		clear(now)
		for _, next := range current {
			if min(row.bottom, next.bottom) > max(row.top, next.top) {
				for token, count := range next.tokens {
					now[token] += count
				}
			}
		}
		missing := 0
		for token, count := range row.tokens {
			missing += max(0, count-now[token])
		}
		if missing == 0 {
			continue
		}
		// Lost OCR is not a replacement, nor is an answer on another line.
		for token, count := range now {
			if count > row.tokens[token] {
				changed += missing
				break
			}
		}
	}
	return changed
}

func countCritical(out map[string]int, token string) map[string]int {
	if out == nil {
		out = make(map[string]int)
	}
	out[token]++
	return out
}

// OCR joins equations, so count literals instead of its word groups.
func addCritical(out map[string]int, word string) map[string]int {
	number := -1
	for i, r := range word {
		if unicode.IsDigit(r) {
			if number < 0 {
				number = i
			}
			continue
		}
		if number >= 0 && (r == '.' || r == ':') && i+1 < len(word) {
			next, _ := utf8.DecodeRuneInString(word[i+1:])
			if unicode.IsDigit(next) {
				continue
			}
		}
		if number >= 0 {
			out = countCritical(out, word[number:i])
			number = -1
		}
		if strings.ContainsRune("+-*/=^×÷√<>≤≥%", r) {
			out = countCritical(out, word[i:i+utf8.RuneLen(r)])
		}
	}
	if number >= 0 {
		out = countCritical(out, word[number:])
	}
	return out
}

var labelRe = regexp.MustCompile(`(?i)^(?:problem|question|exercise|task|q|#)\.?\s*(\d+[a-z]?)$`)

// label is the view's problem label ("Problem 4", "Q4", "#4"), or "".
func label(words []screenask.Word) string {
	previous := ""
	for _, word := range words {
		for _, token := range tokens(word.Text) {
			if match := labelRe.FindStringSubmatch(token); match != nil {
				return match[1]
			}
			if previous != "" && labelRe.MatchString(previous+" "+token) {
				return token
			}
			previous = token
		}
	}
	return ""
}
