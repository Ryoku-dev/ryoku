package keep

import (
	"cmp"
	"math"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/screenask"
)

// KeyFor is the key of goal in app: the app in lower case, "::", and the
// goal's slug, its meaningful words in order joined by "-".
func KeyFor(app, goal string) string {
	return strings.ToLower(strings.TrimSpace(app)) + "::" + strings.Join(goalTokens(goal), "-")
}

// filler are the words a request wraps its goal in. Words that change what
// is meant ("as", "on", "off", "in", "out", "not") are never filler.
var filler = map[string]bool{
	"a": true, "an": true, "the": true, "this": true, "that": true, "these": true, "those": true,
	"my": true, "your": true, "our": true, "its": true, "i": true, "me": true, "we": true, "you": true, "it": true,
	"how": true, "do": true, "does": true, "did": true, "can": true, "could": true, "would": true, "should": true,
	"will": true, "to": true, "please": true, "show": true, "tell": true, "help": true, "want": true, "need": true,
	"way": true, "where": true, "what": true, "is": true, "are": true, "be": true, "just": true, "now": true,
	"here": true, "there": true, "some": true, "let": true, "lets": true, "like": true, "walk": true,
	"through": true, "oni": true, "hey": true, "again": true,
}

// goalTokens is the goal's meaningful words: lower case letter and digit
// runs without filler, a plural "s" dropped so "files" and "file" agree.
func goalTokens(goal string) []string {
	var out []string
	for _, w := range words(goal) {
		if filler[w] {
			continue
		}
		if utf8.RuneCountInString(w) > 3 && strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss") {
			w = strings.TrimSuffix(w, "s")
		}
		out = append(out, w)
	}
	return out
}

// similarity is the Dice coefficient, 0..1, of two goals' words and of their
// adjacent pairs, so the same words in another order ("pdf to word", "word
// to pdf") are not the same goal.
func similarity(a, b []string) float64 {
	ua, ub := units(a), units(b)
	if len(ua)+len(ub) == 0 {
		return 0
	}
	return 2 * float64(common(ua, ub)) / float64(len(ua)+len(ub))
}

func units(ws []string) []string {
	out := slices.Clone(ws)
	for i := 1; i < len(ws); i++ {
		out = append(out, ws[i-1]+" "+ws[i])
	}
	return out
}

// common counts the items a and b share, each item at most as often as it
// appears in both.
func common(a, b []string) int {
	left := make(map[string]int, len(a))
	for _, w := range a {
		left[w]++
	}
	n := 0
	for _, w := range b {
		if left[w] > 0 {
			left[w]--
			n++
		}
	}
	return n
}

// words lowercases s and splits it into its letter and digit runs.
func words(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// Score says, 0..100, how well the text found on the screen names the saved
// label: the Dice coefficient of their whole words. A word inside another
// ("Save" in "Autosave") never counts, and a label with more words than the
// one saved ("Save As" for "Save") scores below FindMin.
func Score(saved, found string) int {
	a, b := words(saved), words(found)
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	return 200 * common(a, b) / (len(a) + len(b))
}

// Found is where a replayed step's control was read. Refused names why the
// found control is a private field; a refused control is not found.
type Found struct {
	Box     feature.Box
	Text    string
	Score   int
	Refused string
}

// phraseGap is how many average character widths may separate two words on
// a row for them to read as one label. A space is about one; the padding
// between two buttons or menu items is wider.
const phraseGap = 1.5

// Locate reads where st's control is among words, the text read in the
// window in fractions of it. Words read as one label when they share a row
// and sit a space apart; the label scoring best against st.Label, at least
// FindMin, is found, the nearest to where the step was kept breaking a tie.
// The found box is checked again for a private field from its own text and
// the text beside it, so a control that became one is refused. With no
// label found, the text now read where the control was kept is checked the
// same way: a control relabelled as a private field in its place is refused
// too, not merely missed.
func Locate(st Step, ws []screenask.Word) (Found, bool) {
	var best Found
	bestAt := math.Inf(1)
	for _, p := range phrases(ws) {
		score := Score(st.Label, p.text)
		at := distance(p.box, st.Box)
		if score > best.Score || (score == best.Score && score > 0 && at < bestAt) {
			best, bestAt = Found{Box: p.box, Text: p.text, Score: score}, at
		}
	}
	if best.Score < FindMin {
		if there := placeText(st.Box, ws); there != "" {
			if why, private := screenask.Sensitive(st.Role+" "+there, ws, st.Box); private {
				return Found{Box: st.Box, Text: there, Score: best.Score, Refused: why}, false
			}
		}
		return Found{Score: best.Score}, false
	}
	if why, private := screenask.Sensitive(st.Role+" "+best.Text, ws, best.Box); private {
		best.Refused = why
		return best, false
	}
	return best, true
}

// placeText is the text read inside box, row by row: what sits where a
// control was kept.
func placeText(box feature.Box, ws []screenask.Word) string {
	var in []string
	for _, p := range phrases(ws) {
		cx, cy := p.box.X+p.box.W/2, p.box.Y+p.box.H/2
		if cx >= box.X && cx <= box.X+box.W && cy >= box.Y && cy <= box.Y+box.H {
			in = append(in, p.text)
		}
	}
	return strings.Join(in, " ")
}

// Find is Locate's box and score; ok is false when nothing scored FindMin
// or the found control is a private field.
func Find(st Step, ws []screenask.Word) (feature.Box, int, bool) {
	f, ok := Locate(st, ws)
	if !ok {
		return feature.Box{}, f.Score, false
	}
	return f.Box, f.Score, true
}

type phrase struct {
	text string
	box  feature.Box
}

// phrases joins the words read into labels: row by row, left to right,
// joining neighbours no more than phraseGap character widths apart.
func phrases(ws []screenask.Word) []phrase {
	var kept []screenask.Word
	for _, w := range ws {
		if strings.TrimSpace(w.Text) != "" && w.Box.W > 0 && w.Box.H > 0 && finite(w.Box) {
			kept = append(kept, w)
		}
	}
	slices.SortStableFunc(kept, func(a, b screenask.Word) int { return cmp.Compare(midY(a.Box), midY(b.Box)) })
	var out []phrase
	for len(kept) > 0 {
		top, n := midY(kept[0].Box), 1
		for n < len(kept) && midY(kept[n].Box)-top <= max(kept[0].Box.H, kept[n].Box.H)/2 {
			n++
		}
		row := kept[:n]
		kept = kept[n:]
		slices.SortStableFunc(row, func(a, b screenask.Word) int { return cmp.Compare(a.Box.X, b.Box.X) })
		cur := phrase{text: strings.TrimSpace(row[0].Text), box: row[0].Box}
		for i := 1; i < len(row); i++ {
			prev, w := row[i-1], row[i]
			gap := w.Box.X - (prev.Box.X + prev.Box.W)
			if gap <= phraseGap*min(charWidth(prev), charWidth(w)) {
				cur.text += " " + strings.TrimSpace(w.Text)
				cur.box = union(cur.box, w.Box)
				continue
			}
			out = append(out, cur)
			cur = phrase{text: strings.TrimSpace(w.Text), box: w.Box}
		}
		out = append(out, cur)
	}
	return out
}

func charWidth(w screenask.Word) float64 {
	return w.Box.W / float64(max(1, utf8.RuneCountInString(strings.TrimSpace(w.Text))))
}

func midY(b feature.Box) float64 { return b.Y + b.H/2 }

func union(a, b feature.Box) feature.Box {
	x, y := min(a.X, b.X), min(a.Y, b.Y)
	return feature.Box{X: x, Y: y, W: max(a.X+a.W, b.X+b.W) - x, H: max(a.Y+a.H, b.Y+b.H) - y}
}

func distance(a, b feature.Box) float64 {
	return math.Hypot(a.X+a.W/2-(b.X+b.W/2), a.Y+a.H/2-(b.Y+b.H/2))
}

func finite(b feature.Box) bool {
	for _, v := range []float64{b.X, b.Y, b.W, b.H} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}
