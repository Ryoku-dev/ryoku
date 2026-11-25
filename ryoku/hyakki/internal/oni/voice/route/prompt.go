package route

import (
	"slices"
	"strings"
	"unicode"
)

// MaxLeadWords is the most words the first sentence of anything Oni says
// aloud may hold.
const MaxLeadWords = 5

// The voice block is the spoken-reply discipline. It is not a character's
// to change: it follows the safety rules and the character's persona.
const (
	voiceHead  = "Voice reply rules. This reply is heard, not read. Open with the answer, five words at most."
	voiceShort = "Say one sentence in all unless the user asks for more."
	voiceLong  = "The user asked for more, so a few short sentences may follow; stop when the answer is complete."
	voiceTail  = "Speak in plain words, with no lists, headings, code, links, tables or symbols."
)

// VoiceBlock is the spoken-reply discipline for an Ask turn: a first
// sentence of at most MaxLeadWords words, and one sentence in all unless
// long, the user having asked for more.
func VoiceBlock(long bool) string {
	middle := voiceShort
	if long {
		middle = voiceLong
	}
	return voiceHead + " " + middle + " " + voiceTail
}

// Prompt composes the instructions for a voice Ask turn: the safety rules
// first and unchanged, then the character's persona style block, then the
// voice block. The daemon passes the same safety rules a typed Ask gets; a
// persona or voice block never comes before them.
func Prompt(safety, persona string, long bool) string {
	parts := make([]string, 0, 3)
	for _, p := range []string{strings.TrimSpace(safety), strings.TrimSpace(persona)} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(append(parts, VoiceBlock(long)), "\n\n")
}

// Sentences splits text at a full stop, question mark or exclamation mark
// followed by a space or the end. A stop inside a number ("3.5") or a time
// ("3:45") does not split.
func Sentences(text string) []string {
	var out []string
	rs := []rune(strings.TrimSpace(text))
	start := 0
	for i, r := range rs {
		if r != '.' && r != '!' && r != '?' && r != '…' {
			continue
		}
		if i+1 < len(rs) && !unicode.IsSpace(rs[i+1]) {
			continue
		}
		if s := strings.TrimSpace(string(rs[start : i+1])); s != "" {
			out = append(out, s)
		}
		start = i + 1
	}
	if s := strings.TrimSpace(string(rs[start:])); s != "" {
		out = append(out, s)
	}
	return out
}

// Words counts the spoken words in text: whitespace-separated runs holding
// a letter or digit.
func Words(text string) int {
	n := 0
	for _, f := range strings.Fields(text) {
		if strings.IndexFunc(f, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0 {
			n++
		}
	}
	return n
}

// LeadWords is how many words the first sentence of text holds.
func LeadWords(text string) int {
	s := Sentences(text)
	if len(s) == 0 {
		return 0
	}
	return Words(s[0])
}

// Fits reports whether a line keeps the voice discipline on its own: one
// sentence of at most MaxLeadWords words.
func Fits(line string) bool {
	return len(Sentences(line)) == 1 && LeadWords(line) <= MaxLeadWords
}

// Spoken is the part of a model reply Oni says aloud: the first sentence,
// or the whole reply when the user asked for more. The rest stays on the
// pill.
func Spoken(reply string, long bool) string {
	s := Sentences(reply)
	if len(s) == 0 {
		return ""
	}
	if long {
		return strings.Join(s, " ")
	}
	return s[0]
}

// leadTails are words a spoken step's opening is never left ending on.
var leadTails = []string{
	"a", "an", "the", "to", "in", "on", "at", "of", "for", "from", "with", "by", "and", "or", "but",
	"so", "then", "your", "my", "its", "this", "that", "these", "those", "into", "onto", "near",
	"under", "over", "below", "above", "next", "is", "are", "am", "was", "be", "it", "i", "you", "we",
}

// Lead is what Oni says aloud of a guide step whose card shows it in full:
// its first sentence when that keeps the voice discipline, else its
// opening up to a clause break and at most MaxLeadWords words, cut back so
// it does not end on a word like "the" or "at". A step opens with its
// action, so the action is what is said. Lead is "" when no opening of at
// least two words is left.
func Lead(step string) string {
	s := Sentences(step)
	if len(s) == 0 {
		return ""
	}
	if Words(s[0]) <= MaxLeadWords {
		return s[0]
	}
	var out []string
	spoken := 0
	for _, w := range strings.Fields(s[0]) {
		out = append(out, w)
		if Words(w) > 0 {
			spoken++
		}
		if spoken == MaxLeadWords || strings.ContainsAny(w, ",;:") {
			break
		}
	}
	notWord := func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }
	for len(out) > 0 {
		last := strings.ToLower(strings.TrimFunc(out[len(out)-1], notWord))
		if last != "" && !slices.Contains(leadTails, last) {
			break
		}
		out = out[:len(out)-1]
	}
	if Words(strings.Join(out, " ")) < 2 {
		return ""
	}
	out[len(out)-1] = strings.TrimRightFunc(out[len(out)-1], notWord)
	return strings.Join(out, " ") + "."
}
