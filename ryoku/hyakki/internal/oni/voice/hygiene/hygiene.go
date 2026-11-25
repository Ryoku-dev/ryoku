// Package hygiene cleans speech-to-text transcripts with the spec's
// voice.hygiene numbers: decoder loops collapse, repeated sentences go, and
// a transcript with more words than its audio could hold is dropped.
package hygiene

import (
	"math"
	"strings"
	"sync"
	"unicode"

	"ryoku-rashin/internal/oni/spec"
)

var rules = sync.OnceValue(func() spec.VoiceHygiene {
	s, err := spec.Load()
	if err != nil {
		panic(err)
	}
	return s.Voice.Hygiene
})

// key is a word as compared: lower case, letters and digits only.
func key(w string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, w)
}

// Collapse keeps once any phrase of up to voice.hygiene.loopMaxWords words
// said voice.hygiene.loopRepeats or more times in a row, comparing words
// without case or punctuation; the first copy is the one kept. Whitespace
// is normalised to single spaces.
func Collapse(text string) string {
	h := rules()
	words := strings.Fields(text)
	keys := make([]string, len(words))
	for i, w := range words {
		keys[i] = key(w)
	}
	out := make([]string, 0, len(words))
	for i := 0; i < len(words); {
		next := i + 1
		for n := 1; n <= h.LoopMaxWords && i+n*h.LoopRepeats <= len(words); n++ {
			if !phrase(keys[i : i+n]) {
				continue
			}
			reps := 1
			for i+(reps+1)*n <= len(words) && equal(keys[i:i+n], keys[i+reps*n:i+(reps+1)*n]) {
				reps++
			}
			if reps >= h.LoopRepeats {
				next = i + reps*n
				out = append(out, words[i:i+n]...)
				break
			}
		}
		if next == i+1 {
			out = append(out, words[i])
		}
		i = next
	}
	return strings.Join(out, " ")
}

// phrase reports whether a run of word keys holds any word to compare.
func phrase(keys []string) bool {
	for _, k := range keys {
		if k != "" {
			return true
		}
	}
	return false
}

func equal(a, b []string) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Plausible reports whether secs of audio could hold text: at most
// ceil(secs·wordsPerS) + slackWords words.
func Plausible(text string, secs float64) bool {
	h := rules()
	if !(secs > 0) || math.IsInf(secs, 0) {
		secs = 0
	}
	return len(strings.Fields(text)) <= int(math.Ceil(secs*h.WordsPerS))+h.SlackWords
}

// sentences splits text after each run of sentence marks that is followed
// by a space or the end; each sentence keeps its marks and loses the
// whitespace around it.
func sentences(text string) []string {
	var out []string
	rs := []rune(text)
	from := 0
	for i := 0; i < len(rs); i++ {
		if !strings.ContainsRune(".!?…", rs[i]) {
			continue
		}
		j := i
		for j+1 < len(rs) && strings.ContainsRune(".!?…", rs[j+1]) {
			j++
		}
		if j+1 == len(rs) || unicode.IsSpace(rs[j+1]) {
			if s := strings.TrimSpace(string(rs[from : j+1])); s != "" {
				out = append(out, s)
			}
			from = j + 1
		}
		i = j
	}
	if s := strings.TrimSpace(string(rs[from:])); s != "" {
		out = append(out, s)
	}
	return out
}

// DropRepeatedSentences removes every sentence of at least
// voice.hygiene.sentenceMinWords words that repeats an earlier sentence,
// comparing words without case or punctuation. Shorter sentences ("No.
// No.") are kept. Sentences are joined by single spaces.
func DropRepeatedSentences(text string) string {
	h := rules()
	seen := map[string]bool{}
	var out []string
	for _, s := range sentences(text) {
		var ks []string
		for _, w := range strings.Fields(s) {
			if k := key(w); k != "" {
				ks = append(ks, k)
			}
		}
		if len(ks) >= h.SentenceMinWords {
			k := strings.Join(ks, " ")
			if seen[k] {
				continue
			}
			seen[k] = true
		}
		out = append(out, s)
	}
	return strings.Join(out, " ")
}

// Tidy runs the whole clean-up on a transcript of secs of audio: loops
// collapse, repeated sentences go, then an implausible or empty transcript
// is dropped (ok false).
func Tidy(text string, secs float64) (string, bool) {
	t := DropRepeatedSentences(Collapse(text))
	if t == "" || !Plausible(t, secs) {
		return "", false
	}
	return t, true
}
