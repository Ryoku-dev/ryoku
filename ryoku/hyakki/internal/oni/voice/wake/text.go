package wake

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// token is one word of a transcript as compared, and the byte offset in the
// transcript just past it.
type token struct {
	word string
	end  int
}

// strip replaces a decoder's annotations, such as "[BLANK_AUDIO]",
// "(wind blowing)" or "*coughs*", with a space. An annotation left open runs
// to the end of the text.
func strip(text string) string {
	if !strings.ContainsAny(text, "[(*") {
		return text
	}
	var b strings.Builder
	var close rune
	for _, r := range text {
		switch {
		case close != 0:
			if r == close {
				close = 0
			}
		case r == '[':
			close = ']'
		case r == '(':
			close = ')'
		case r == '*':
			close = '*'
		default:
			b.WriteRune(r)
			continue
		}
		b.WriteByte(' ')
	}
	return b.String()
}

// tokens splits text with no annotations into words as compared: accents
// folded, lower case, apostrophes dropped ("what's" is "whats"), anything
// else that is not a letter or digit a separator.
func tokens(text string) []token {
	var out []token
	var b strings.Builder
	flush := func(end int) {
		if b.Len() > 0 {
			out = append(out, token{word: b.String(), end: end})
			b.Reset()
		}
	}
	for i, r := range text {
		switch {
		case r == '\'' || r == '’':
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if r < unicode.MaxASCII {
				b.WriteRune(unicode.ToLower(r))
				continue
			}
			for _, f := range norm.NFKD.String(string(r)) {
				if !unicode.Is(unicode.Mn, f) {
					b.WriteRune(unicode.ToLower(f))
				}
			}
		default:
			flush(i)
		}
	}
	flush(len(text))
	return out
}

// Normalize is a transcript as the wake and routing matchers compare it:
// annotations dropped, its words folded as above and joined by single
// spaces.
func Normalize(text string) string {
	toks := tokens(strip(text))
	words := make([]string, len(toks))
	for i, t := range toks {
		words[i] = t.word
	}
	return strings.Join(words, " ")
}

// after is what follows the first n tokens of text, with the separators that
// joined it to them trimmed.
func after(text string, toks []token, n int) string {
	if n > 0 {
		text = text[toks[n-1].end:]
	}
	return strings.TrimSpace(strings.TrimLeftFunc(text, func(c rune) bool {
		return unicode.IsSpace(c) || strings.ContainsRune(",.!?;:-–—…", c)
	}))
}
