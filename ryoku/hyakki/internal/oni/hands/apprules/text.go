package apprules

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"ryoku-rashin/internal/sensitive"
)

// MaxChunk is the most runes one typing call carries.
const MaxChunk = 40

// Text judges what Hands is asked to type. Text with a line break is the
// person's to send; text with any other control or format character, and
// text shaped like a secret (sensitive.Secret: a private-field word, a key,
// a token, a one-time code, a card or identity number), is refused;
// anything else may be typed. The length bound is the caller's (the spec's
// hands.app.maxTypeRunes).
func Text(s string) Verdict {
	if !utf8.ValidString(s) {
		return Verdict{KindRefused, WhyControl}
	}
	if strings.TrimSpace(s) == "" {
		return Verdict{KindRefused, WhyEmpty}
	}
	broken := false
	for _, r := range s {
		switch {
		case r == '\n' || r == '\r' || r == '\u2028' || r == '\u2029' || r == '\u0085':
			broken = true
		case r == zwj || r == '\u200c' || r >= 0xe0020 && r <= 0xe007f:
			// joiners and emoji tags shape what is typed; they press nothing
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r):
			return Verdict{KindRefused, WhyControl}
		}
	}
	if sensitive.Secret(s) {
		return Verdict{KindRefused, WhySecret}
	}
	if broken {
		return Verdict{KindHandoff, WhySubmit}
	}
	return Verdict{KindSafe, WhySafe}
}

// Chunks cuts text into pieces of at most max runes for one typing call
// each, never inside a cluster of combining marks, joined emoji or a flag;
// a cluster longer than max is cut on rune boundaries. A max below one is
// one.
func Chunks(text string, max int) []string {
	if max < 1 {
		max = 1
	}
	var out []string
	start, n := 0, 0
	for i := 0; i < len(text); {
		end, runes := cluster(text, i)
		if n > 0 && n+runes > max {
			out = append(out, text[start:i])
			start, n = i, 0
		}
		if runes > max {
			k := 0
			for j := i; j < end; {
				if k == max {
					out = append(out, text[start:j])
					start, k = j, 0
				}
				_, size := utf8.DecodeRuneInString(text[j:])
				j += size
				k++
			}
			n, i = k, end
			continue
		}
		n += runes
		i = end
	}
	if start < len(text) {
		out = append(out, text[start:])
	}
	return out
}

const zwj = '\u200d'

// cluster returns the end of the cluster starting at byte i and its length
// in runes.
func cluster(s string, i int) (int, int) {
	r, size := utf8.DecodeRuneInString(s[i:])
	end, n := i+size, 1
	prev := r
	flags := 0
	if regional(r) {
		flags = 1
	}
	for end < len(s) {
		next, size := utf8.DecodeRuneInString(s[end:])
		switch {
		case prev == zwj, extends(next):
		case regional(next) && flags == 1:
			flags = 2
		default:
			return end, n
		}
		end += size
		n++
		prev = next
	}
	return end, n
}

func extends(r rune) bool {
	switch {
	case r == zwj,
		unicode.In(r, unicode.Mn, unicode.Me, unicode.Mc),
		r >= 0xfe00 && r <= 0xfe0f,
		r >= 0xe0100 && r <= 0xe01ef,
		r >= 0x1f3fb && r <= 0x1f3ff,
		r >= 0xe0020 && r <= 0xe007f:
		return true
	}
	return false
}

func regional(r rune) bool { return r >= 0x1f1e6 && r <= 0x1f1ff }
