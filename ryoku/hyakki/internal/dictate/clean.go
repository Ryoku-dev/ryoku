package dictate

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	i18n "ryoku-i18n"
)

// cleaned is the Clean text and what Polished needs to check a rewrite.
type cleaned struct {
	text      string
	protected []string // protected values left in text, one per occurrence
	snippet   bool     // text is a snippet, typed as stored
	notes     []Note
}

// fillers are sounds a speaker makes while thinking. Words that carry
// meaning in some sentences ("like", "so", "you know") are left alone.
var fillers = map[string]bool{
	"um": true, "umm": true, "uh": true, "uhh": true, "uhm": true,
	"er": true, "erm": true, "hmm": true, "hm": true, "mm": true, "mmm": true,
}

// clean applies the Clean rules in order: fillers, repairs, snippets, the
// dictionary, spoken punctuation, mark tidying, the style's casing and end,
// then capitals.
func clean(text string, p Personal, style Style) cleaned {
	var c cleaned
	toks := tokenize(text)

	removed := 0
	toks = slices.DeleteFunc(toks, func(t token) bool {
		if t.kind == kWord && fillers[strings.ToLower(t.text)] && !allUpper(t.text) {
			removed++
			return true
		}
		return false
	})
	if removed > 0 {
		c.notes = append(c.notes, Note{NoteFillers, i18n.Tf("Removed %d filler sounds.", removed)})
	}

	toks, repaired := repairs(toks)
	for _, marker := range repaired {
		c.notes = append(c.notes, Note{NoteRepair, i18n.Tf("Applied the spoken correction “%s”.", marker)})
	}

	if body, ok := p.snippetFor(toks); ok {
		c.text, c.snippet = body, true
		c.notes = append(c.notes, Note{NoteSnippet, i18n.T("Typed one of your snippets.")})
		return c
	}

	toks, spelled := p.applyDictionary(toks)
	if spelled > 0 {
		c.notes = append(c.notes, Note{NoteDictionary, i18n.Tf("Used your spelling for %d words.", spelled)})
	}

	toks, marked := spokenMarks(toks)
	if marked > 0 {
		c.notes = append(c.notes, Note{NotePunctuation, i18n.T("Turned spoken punctuation into marks.")})
	}

	toks = tidyMarks(toks)
	toks = applyEnd(toks, style.End)
	toks = applyCase(toks, style.Casing)
	for _, t := range toks {
		if t.kind == kProtected {
			c.protected = append(c.protected, t.text)
		}
	}
	c.text = render(toks)
	return c
}

func allUpper(s string) bool {
	letters := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			if !unicode.IsUpper(r) {
				return false
			}
			letters++
		}
	}
	return letters > 1
}

// Repair markers. Each acts only inside the utterance being processed:
// text typed before it is never reached.
const (
	markScratch  = "scratch that"
	markMake     = "make that"
	markMean     = "no, I mean"
	markActually = "actually"
)

// matchMarker reports the repair marker starting at i and the index after it.
func matchMarker(toks []token, i int) (string, int) {
	at := func(k int, w string) bool { return k < len(toks) && toks[k].is(w) }
	switch {
	case at(i, "scratch") && at(i+1, "that"):
		return markScratch, i + 2
	case at(i, "make") && at(i+1, "that"):
		return markMake, i + 2
	case at(i, "no"):
		k := i + 1
		if k < len(toks) && toks[k].mark(",") {
			k++
		}
		if at(k, "i") && at(k+1, "mean") {
			return markMean, k + 2
		}
	case at(i, "actually"):
		return markActually, i + 1
	}
	return "", 0
}

// stopwords cannot be a whole replacement: "no, I mean it" is emphasis, not
// a correction.
var stopwords = map[string]bool{
	"it": true, "that": true, "this": true, "so": true, "yes": true, "no": true,
	"well": true, "okay": true, "ok": true, "right": true,
}

// maxReplacement is the most words a "make that" style correction may carry.
const maxReplacement = 6

// repairs applies spoken corrections left to right.
//
//   - "scratch that", with a pause or stop on at least one side, deletes the
//     sentence before it.
//   - "make that" and "no, I mean", after a pause or stop, and "actually"
//     with a pause or dash on both sides, replace the end of the clause
//     before them with the words after them, up to the next mark: from the
//     replacement's first word when the clause holds it, else as many words
//     as the replacement has.
//
// A marker with nothing before it in this utterance replaces nothing.
func repairs(toks []token) ([]token, []string) {
	var applied []string
	for i := 0; i < len(toks); i++ {
		marker, end := matchMarker(toks, i)
		if marker == "" {
			continue
		}
		prevEdge := i == 0 || boundary(toks[i-1])
		nextEdge := end == len(toks) || boundary(toks[end])

		// back is where the marks just before the marker begin.
		back := i
		for back > 0 && toks[back-1].kind == kPunct && boundary(toks[back-1]) {
			back--
		}

		if marker == markScratch {
			if !prevEdge && !nextEdge {
				continue
			}
			start := back
			for start > 0 && !sentenceEnd(toks[start-1]) {
				start--
			}
			stop := end
			if stop < len(toks) && toks[stop].mark(edgeMarks) {
				stop++
			}
			toks = slices.Delete(toks, start, stop)
			if start < len(toks) {
				toks[start].space = start > 0
			}
			applied = append(applied, marker)
			i = start - 1
			continue
		}

		if i == 0 || !prevEdge {
			continue
		}
		if marker == markActually && (!toks[i-1].mark(pauses+dashes) || end == len(toks) || !toks[end].mark(pauses+dashes)) {
			continue
		}
		from := end
		if from < len(toks) && toks[from].mark(pauses+dashes) {
			from++
		}
		to := from
		for to < len(toks) && !boundary(toks[to]) {
			to++
		}
		var repl []int
		for k := from; k < to; k++ {
			if toks[k].content() {
				repl = append(repl, k)
			}
		}
		if len(repl) == 0 || len(repl) > maxReplacement || len(repl) == 1 && stopwords[strings.ToLower(toks[repl[0]].text)] {
			continue
		}
		clauseStart := back
		for clauseStart > 0 && !boundary(toks[clauseStart-1]) {
			clauseStart--
		}
		var clause []int
		for k := clauseStart; k < back; k++ {
			if toks[k].content() {
				clause = append(clause, k)
			}
		}
		if len(clause) == 0 {
			continue
		}
		target := -1
		first := strings.ToLower(toks[repl[0]].text)
		for k := len(clause) - 1; k >= 0; k-- {
			if strings.ToLower(toks[clause[k]].text) == first {
				target = clause[k]
				break
			}
		}
		if target < 0 {
			if len(repl) >= len(clause) {
				target = clause[0]
			} else {
				target = clause[len(clause)-len(repl)]
			}
		}
		replacement := slices.Clone(toks[from:to])
		replacement[0].space = toks[target].space
		out := make([]token, 0, len(toks))
		out = append(out, toks[:target]...)
		out = append(out, replacement...)
		out = append(out, toks[to:]...)
		toks = out
		applied = append(applied, marker)
		i = target + len(replacement) - 1
	}
	return toks, applied
}

// contentWords lowercases the content tokens, skipping marks and breaks.
func contentWords(toks []token) []string {
	var out []string
	for _, t := range toks {
		if t.content() {
			out = append(out, strings.ToLower(t.text))
		}
	}
	return out
}

// snippetFor returns the snippet the utterance asks for: its trigger said
// alone, or after "insert" or "type".
func (p Personal) snippetFor(toks []token) (string, bool) {
	said := contentWords(toks)
	if len(said) > 1 && (said[0] == "insert" || said[0] == "type") {
		if body, ok := p.snippetMatching(said[1:]); ok {
			return body, true
		}
	}
	return p.snippetMatching(said)
}

func (p Personal) snippetMatching(said []string) (string, bool) {
	if len(said) == 0 {
		return "", false
	}
	for _, s := range p.Snippets {
		if trigger, ok := words(s.Trigger); ok && slices.Equal(trigger, said) && s.Text != "" {
			return s.Text, true
		}
	}
	return "", false
}

// applyDictionary swaps each spoken phrase for its written form, longest
// phrase first. The written form becomes a fixed token no later rule changes.
func (p Personal) applyDictionary(toks []token) ([]token, int) {
	type phrase struct {
		said    []string
		written string
	}
	var phrases []phrase
	for _, e := range p.Dictionary {
		if said, ok := words(e.Spoken); ok && strings.TrimSpace(e.Written) != "" {
			phrases = append(phrases, phrase{said, e.Written})
		}
	}
	if len(phrases) == 0 {
		return toks, 0
	}
	slices.SortStableFunc(phrases, func(a, b phrase) int { return len(b.said) - len(a.said) })
	n := 0
	var out []token
	for i := 0; i < len(toks); {
		matched := false
		for _, ph := range phrases {
			if i+len(ph.said) > len(toks) {
				continue
			}
			ok := true
			for k, w := range ph.said {
				t := toks[i+k]
				if (t.kind != kWord && t.kind != kProtected) || strings.ToLower(t.text) != w {
					ok = false
					break
				}
			}
			if ok {
				out = append(out, token{text: strings.TrimSpace(ph.written), kind: kFixed, space: toks[i].space})
				i += len(ph.said)
				n++
				matched = true
				break
			}
		}
		if !matched {
			out = append(out, toks[i])
			i++
		}
	}
	return out, n
}

// spoken punctuation: the words, and the mark or break each becomes.
var spokenTable = []struct {
	said []string
	mark string
	kind kind
}{
	{[]string{"question", "mark"}, "?", kPunct},
	{[]string{"exclamation", "mark"}, "!", kPunct},
	{[]string{"exclamation", "point"}, "!", kPunct},
	{[]string{"full", "stop"}, ".", kPunct},
	{[]string{"new", "paragraph"}, "\n\n", kBreak},
	{[]string{"new", "line"}, "\n", kBreak},
	{[]string{"period"}, ".", kPunct},
	{[]string{"comma"}, ",", kPunct},
	{[]string{"semicolon"}, ";", kPunct},
	{[]string{"colon"}, ":", kPunct},
}

// determiners before a punctuation word make it a noun: "the period", "a
// new line".
var determiners = map[string]bool{
	"the": true, "a": true, "an": true, "this": true, "that": true, "these": true,
	"those": true, "each": true, "every": true, "one": true, "per": true, "any": true,
	"some": true, "no": true, "our": true, "your": true, "my": true, "his": true,
	"her": true, "their": true, "its": true,
}

// spokenMarks turns spoken punctuation into marks. A mark needs words before
// it and is not a noun ("the period", "a comma", "period of"); marks the
// transcriber already put beside the spoken word are dropped for it.
func spokenMarks(toks []token) ([]token, int) {
	n := 0
	for i := 0; i < len(toks); i++ {
		var mark string
		var mk kind
		width := 0
		for _, row := range spokenTable {
			if i+len(row.said) > len(toks) {
				continue
			}
			ok := true
			for k, w := range row.said {
				if !toks[i+k].is(w) {
					ok = false
					break
				}
			}
			if ok {
				mark, mk, width = row.mark, row.kind, len(row.said)
				break
			}
		}
		if width == 0 {
			continue
		}
		if i > 0 && toks[i-1].kind == kWord && determiners[strings.ToLower(toks[i-1].text)] {
			continue
		}
		if i+width < len(toks) && toks[i+width].is("of") {
			continue
		}
		if mk == kPunct {
			before := i - 1
			for before >= 0 && toks[before].kind == kPunct {
				before--
			}
			if before < 0 || toks[before].kind == kBreak {
				continue
			}
		}
		// A spoken mark replaces the transcriber's marks beside it; a spoken
		// break keeps the stop that ended the sentence before it.
		left, right := i, i+width
		beside := pauses + stops
		if mk == kBreak {
			beside = pauses
		}
		if left > 0 && toks[left-1].mark(beside) {
			left--
		}
		if right < len(toks) && toks[right].mark(pauses+stops) {
			right++
		}
		toks = slices.Replace(toks, left, right, token{text: mark, kind: mk})
		n++
		i = left
	}
	return toks, n
}

// tidyMarks drops pauses with nothing to pause after, doubled pauses, a pause
// before a stop and a trailing comma or semicolon.
func tidyMarks(toks []token) []token {
	out := make([]token, 0, len(toks))
	for _, t := range toks {
		var last token
		if len(out) > 0 {
			last = out[len(out)-1]
		}
		if t.mark(pauses) && (len(out) == 0 || last.kind == kBreak || last.mark(pauses+stops)) {
			continue
		}
		if t.mark(stops) && last.mark(pauses) {
			out[len(out)-1] = t
			continue
		}
		out = append(out, t)
	}
	for len(out) > 0 && out[len(out)-1].mark(",;") {
		out = out[:len(out)-1]
	}
	return out
}

// Style casing and end rules.
const (
	CasingSentence = "sentence" // capital at each sentence start (the default)
	CasingKeep     = "keep"     // capitals as transcribed
	CasingLower    = "lower"    // lower case at each sentence start
	EndKeep        = "keep"     // the end as transcribed (the default)
	EndPeriod      = "period"   // end with a full stop when none ends it
	EndNone        = "none"     // drop a final full stop
)

// lastContent is the index of the last token that is not a line break.
func lastContent(toks []token) int {
	k := len(toks) - 1
	for k >= 0 && toks[k].kind == kBreak {
		k--
	}
	return k
}

func applyEnd(toks []token, end string) []token {
	k := lastContent(toks)
	if k < 0 {
		return toks
	}
	switch end {
	case EndPeriod:
		if toks[k].content() {
			return slices.Insert(toks, k+1, token{text: ".", kind: kPunct})
		}
	case EndNone:
		if toks[k].mark(".") && (k == 0 || !toks[k-1].mark(".")) {
			return slices.Delete(toks, k, k+1)
		}
	}
	return toks
}

// abbreviations end in a full stop that does not end the sentence.
var abbreviations = map[string]bool{
	"e.g": true, "i.e": true, "vs": true, "mr": true, "mrs": true, "ms": true,
	"dr": true, "st": true, "jr": true, "sr": true, "prof": true, "approx": true, "cf": true,
}

// abbreviation reports a full stop at k that closes an abbreviation.
func abbreviation(toks []token, k int) bool {
	return k > 0 && toks[k-1].kind == kWord && abbreviations[strings.ToLower(toks[k-1].text)]
}

// ellipsis reports a full stop at k that is part of a run of dots.
func ellipsis(toks []token, k int) bool {
	return k > 0 && toks[k-1].mark(".") || k+1 < len(toks) && toks[k+1].mark(".")
}

// applyCase sets capitals at sentence starts by the casing rule and writes
// the pronoun "I" as a capital. Protected values and dictionary spellings
// are never changed.
func applyCase(toks []token, casing string) []token {
	if casing == CasingKeep {
		return toks
	}
	start := true
	for k := range toks {
		t := &toks[k]
		switch {
		case t.kind == kBreak:
			start = true
		case t.mark(stops):
			start = t.text != "." || !abbreviation(toks, k) && !ellipsis(toks, k)
		case t.kind == kWord:
			if pronounI(t.text) {
				t.text = "I" + t.text[1:]
			} else if start && casing == CasingLower {
				if !allUpper(t.text) {
					t.text = withFirst(t.text, unicode.ToLower)
				}
			} else if start {
				t.text = withFirst(t.text, unicode.ToUpper)
			}
			start = false
		case t.content():
			start = false
		}
	}
	return toks
}

// pronounI reports the pronoun "I" in either case and its contractions:
// i'm, i'll, i've, i'd.
func pronounI(w string) bool {
	if w == "i" || w == "I" {
		return true
	}
	for _, rest := range []string{"'m", "'ll", "'ve", "'d", "’m", "’ll", "’ve", "’d"} {
		if strings.EqualFold(w, "i"+rest) {
			return true
		}
	}
	return false
}

func withFirst(s string, to func(rune) rune) string {
	r, size := utf8.DecodeRuneInString(s)
	return string(to(r)) + s[size:]
}
