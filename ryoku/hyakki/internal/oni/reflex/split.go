package reflex

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
)

// Kinds of cut, from the strongest: the end of a sentence, a joining word
// or comma, and a command word met in the middle of a clause.
const (
	cutSentence = iota
	cutJoint
	cutVerb
)

var (
	sentenceCut = regexp.MustCompile(`[.!?;]+\s+`)
	jointCut    = regexp.MustCompile(`(?i)\s*,?\s*\b(?:and then|and also|and after that|and|then|also|after that)\b\s*|\s*,\s*`)
	verbCut     = regexp.MustCompile(`(?i)\b(?:open|launch|start|search|look up|turn|mute|unmute|pause|play|resume|take|set|remind|snap|(?:mini|maxi)mi[sz]e|close|go to|run|screenshot|new tab)\b`)
)

type cut struct{ start, end, kind int }

type span struct{ s, e int }

// Split cuts an utterance into clauses. It cuts at sentence ends, at "and",
// "then" and commas, and where a command word starts in the middle of a
// clause, but only where the cut leaves two clauses the table claims: so
// "search for salt and pepper" stays whole while "open files and turn it
// up" becomes two. A note or a reminder keeps the rest of its sentence; a
// search, a timer or a named routine keeps the command words after it. A
// pair the table reads better as one clause ("play pause") stays one.
func Split(text string) []string {
	text = clean(text)
	if text == "" {
		return nil
	}
	var cuts []cut
	for _, ix := range sentenceCut.FindAllStringIndex(text, -1) {
		cuts = append(cuts, cut{ix[0], ix[1], cutSentence})
	}
	for _, ix := range jointCut.FindAllStringIndex(text, -1) {
		cuts = append(cuts, cut{ix[0], ix[1], cutJoint})
	}
	for _, ix := range verbCut.FindAllStringIndex(text, -1) {
		if ix[0] > 0 {
			cuts = append(cuts, cut{ix[0], ix[0], cutVerb})
		}
	}
	slices.SortStableFunc(cuts, func(a, b cut) int {
		return cmp.Or(cmp.Compare(a.start, b.start), cmp.Compare(a.kind, b.kind))
	})
	kept := cuts[:0]
	end := -1
	for _, c := range cuts {
		if c.start < end || (c.kind == cutVerb && c.start <= end) {
			continue
		}
		kept = append(kept, c)
		end = c.end
	}

	segs := make([]span, 0, len(kept)+1)
	from := 0
	for _, c := range kept {
		segs = append(segs, trimSpan(text, from, c.start))
		from = c.end
	}
	segs = append(segs, trimSpan(text, from, len(text)))

	var out []string
	cur := segs[0]
	for i := 1; i < len(segs); i++ {
		seg := segs[i]
		if seg.s >= seg.e {
			continue
		}
		if cur.s >= cur.e {
			cur = seg
			continue
		}
		// The clause after the cut is the shortest run of pieces from here
		// that the table claims: "then take a | screenshot" reads as one.
		right, j := seg, i
		for ; j < len(segs); j++ {
			if segs[j].s < segs[j].e {
				right.e = segs[j].e
			}
			if _, _, ok := judge(text[right.s:right.e], nil); ok {
				break
			}
		}
		if j < len(segs) && divides(text[cur.s:cur.e], text[right.s:right.e], text[cur.s:right.e], kept[i-1].kind) {
			out = append(out, text[cur.s:cur.e])
			cur, i = right, j
			continue
		}
		cur.e = seg.e
	}
	if cur.s < cur.e {
		out = append(out, text[cur.s:cur.e])
	}
	return out
}

// trimSpan is text[s:e] without the stray marks at either end.
func trimSpan(text string, s, e int) span {
	t := text[s:e]
	l := len(t) - len(strings.TrimLeft(t, edge))
	r := len(strings.TrimRight(t, edge))
	if r <= l {
		return span{s, s}
	}
	return span{s + l, s + r}
}

// divides reports whether a cut of kind between left and right makes two
// clauses: right is claimed, and unless the cut ends a sentence, left is a
// complete clause that does not reach over this kind of cut and the two
// together are not one narrower clause.
func divides(left, right, whole string, kind int) bool {
	if _, _, ok := judge(right, nil); !ok {
		return false
	}
	if kind == cutSentence {
		return true
	}
	lv, lr, ok := judge(left, nil)
	if !ok || lv.verb != "" {
		return false
	}
	need := absorbVerbs
	if kind == cutJoint {
		need = absorbJoints
	}
	if lr.absorb >= need {
		return false
	}
	if wv, wr, ok := judge(whole, nil); ok && wv.verb == "" && !wr.greedy {
		return false
	}
	return true
}
