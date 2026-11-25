package screenask

import (
	"cmp"
	"slices"
	"strings"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/sensitive"
)

// Private fields are refused in code, on every path that could point at one,
// including a point replayed without a model. The check reads only the
// field's label, its role and the text read beside it; the value in the field
// is never looked at or kept. With no text reader the label and the control
// list are all there is, so an unlabelled private field is not caught. The
// words that name a private field are sensitive.Label's, shared with
// dictation, Hands and memory.

// Reach of the text read beside a field: labels sit to its left or above it.
const (
	reachLeft  = 0.12
	reachAbove = 0.04
	reachEdge  = 0.02
)

// Sensitive says whether a point at box with label is at a private field,
// judged from the label and from the text read in or beside the box (to its
// left or just above it, where a field's caption sits). reason names what
// gave it away.
func Sensitive(label string, ocr []Word, box feature.Box) (reason string, refused bool) {
	if what := sensitive.Label(label); what != "" {
		return "the label names " + what, true
	}
	near := feature.Box{
		X: box.X - max(box.W, reachLeft),
		Y: box.Y - max(1.5*box.H, reachAbove),
	}
	near.W = box.X + box.W + reachEdge - near.X
	near.H = box.Y + box.H + reachEdge - near.Y
	var beside []Word
	for _, w := range ocr {
		if contains(near, center(w.Box)) {
			beside = append(beside, w)
		}
	}
	for _, row := range rows(beside) {
		if what := sensitive.Label(row); what != "" {
			return "the text beside it names " + what, true
		}
	}
	return "", false
}

// rows reads the words row by row, left to right, so a caption split into
// several words still reads as one phrase while words on different rows never
// join. Words whose centres sit within half a word's height of a row's first
// word share that row.
func rows(words []Word) []string {
	slices.SortStableFunc(words, func(a, b Word) int { return cmp.Compare(center(a.Box).y, center(b.Box).y) })
	var out []string
	for len(words) > 0 {
		top := center(words[0].Box).y
		n := 1
		for n < len(words) && center(words[n].Box).y-top <= max(words[0].Box.H, words[n].Box.H)/2 {
			n++
		}
		row := words[:n]
		slices.SortStableFunc(row, func(a, b Word) int { return cmp.Compare(a.Box.X, b.Box.X) })
		text := make([]string, n)
		for i, w := range row {
			text[i] = w.Text
		}
		out = append(out, strings.Join(text, " "))
		words = words[n:]
	}
	return out
}
